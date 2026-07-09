package conversion

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cdoan1/hyperfleet-api-codegen/pkg/registry"
)

// Generator generates REST types and conversion functions from CRD types
type Generator struct {
	APIVersion string   // e.g., "v1alpha1"
	CRDPackage string   // Import path to CRD types
	OutputDir  string   // Output directory for generated code
	InputDirs  []string // Directories containing CRD source files

	// Internal state
	knownTypes map[string]bool      // Set of all type names
	typeInfos  map[string]*typeInfo // Type name -> type information
}

// typeInfo holds parsed information about a Go type
type typeInfo struct {
	Name       string
	StructType *ast.StructType
	Doc        *ast.CommentGroup
	Fields     []*fieldInfo
}

// fieldInfo holds information about a struct field
type fieldInfo struct {
	GoName    string          // Go field name (e.g., "DisplayName")
	JSONName  string          // JSON tag name (e.g., "displayName")
	GoType    string          // Go type as string (e.g., "string", "*bool")
	FieldPath string          // Registry path (e.g., "spec.displayName")
	Field     *ast.Field      // Original AST field
	Doc       *ast.CommentGroup
	Hidden    bool            // From registry
	WriteMode registry.WriteMode
}

// NewGenerator creates a new conversion generator
func NewGenerator(apiVersion, crdPackage string, inputDirs []string, outputDir string) *Generator {
	return &Generator{
		APIVersion: apiVersion,
		CRDPackage: crdPackage,
		InputDirs:  inputDirs,
		OutputDir:  outputDir,
		knownTypes: make(map[string]bool),
		typeInfos:  make(map[string]*typeInfo),
	}
}

// Generate runs all three generation phases
func (g *Generator) Generate() error {
	// Parse CRD types first
	if err := g.parseTypes(); err != nil {
		return fmt.Errorf("parsing types: %w", err)
	}

	// Phase 1: Generate REST types (filter hidden fields)
	if err := g.generateRESTTypes(); err != nil {
		return fmt.Errorf("generating REST types: %w", err)
	}

	// Phase 2: Generate ServiceSetFields
	if err := g.generateServiceSetFields(); err != nil {
		return fmt.Errorf("generating ServiceSetFields: %w", err)
	}

	// Phase 3: Generate conversion functions
	if err := g.generateConversionFunctions(); err != nil {
		return fmt.Errorf("generating conversion functions: %w", err)
	}

	return nil
}

// parseTypes scans input directories and parses all CRD types
func (g *Generator) parseTypes() error {
	for _, dir := range g.InputDirs {
		fset := token.NewFileSet()

		// Parse all Go files in directory
		//nolint:staticcheck // ParseDir is sufficient for our use case
		pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
			name := fi.Name()
			// Skip test files and generated files
			return !strings.HasSuffix(name, "_test.go") &&
				!strings.HasPrefix(name, "zz_generated")
		}, parser.ParseComments)

		if err != nil {
			return fmt.Errorf("parsing directory %s: %w", dir, err)
		}

		// Collect all types
		for _, pkg := range pkgs {
			for _, file := range pkg.Files {
				for _, decl := range file.Decls {
					genDecl, ok := decl.(*ast.GenDecl)
					if !ok || genDecl.Tok != token.TYPE {
						continue
					}

					for _, spec := range genDecl.Specs {
						typeSpec, ok := spec.(*ast.TypeSpec)
						if !ok || !typeSpec.Name.IsExported() {
							continue
						}

						structType, ok := typeSpec.Type.(*ast.StructType)
						if !ok {
							continue
						}

						typeName := typeSpec.Name.Name
						g.knownTypes[typeName] = true

						// Create type info
						ti := &typeInfo{
							Name:       typeName,
							StructType: structType,
							Doc:        genDecl.Doc,
							Fields:     []*fieldInfo{},
						}

						// Parse fields
						for _, field := range structType.Fields.List {
							// Skip embedded fields
							if len(field.Names) == 0 {
								continue
							}

							for _, name := range field.Names {
								// Skip unexported fields
								if !name.IsExported() {
									continue
								}

								fi := g.parseField(typeName, field, name)
								if fi != nil {
									ti.Fields = append(ti.Fields, fi)
								}
							}
						}

						g.typeInfos[typeName] = ti
					}
				}
			}
		}
	}

	return nil
}

// parseField parses a single struct field
func (g *Generator) parseField(typeName string, field *ast.Field, name *ast.Ident) *fieldInfo {
	goName := name.Name
	jsonName := g.extractJSONTag(field)
	if jsonName == "" || jsonName == "-" {
		return nil // Skip fields without JSON tags
	}

	// Build field path for registry lookup
	fieldPath := g.buildFieldPath(typeName, jsonName)

	// Lookup in registry
	meta, exists := registry.FieldRegistry[fieldPath]

	fi := &fieldInfo{
		GoName:   goName,
		JSONName: jsonName,
		GoType:   g.exprToString(field.Type),
		Field:    field,
		Doc:      field.Doc,
	}

	if exists {
		fi.FieldPath = meta.FieldPath
		fi.Hidden = meta.Hidden
		fi.WriteMode = meta.WriteMode
	}

	return fi
}

// buildFieldPath constructs the registry path for a field
func (g *Generator) buildFieldPath(typeName, jsonName string) string {
	// Map type names to registry prefixes
	switch {
	case strings.HasSuffix(typeName, "Spec"):
		return "spec." + jsonName
	case strings.HasSuffix(typeName, "Status"):
		return "status." + jsonName
	case strings.Contains(typeName, "Passthrough"):
		// For passthrough types, need to determine prefix
		// e.g., HostedClusterSpecPassthrough -> "spec.hostedCluster."
		if strings.HasPrefix(typeName, "HostedCluster") {
			return "spec.hostedCluster." + jsonName
		}
		if strings.HasPrefix(typeName, "NodePool") {
			return "spec.nodePool." + jsonName
		}
		return jsonName
	default:
		return jsonName
	}
}

// extractJSONTag extracts the JSON tag from a field
func (g *Generator) extractJSONTag(field *ast.Field) string {
	if field.Tag == nil {
		return ""
	}

	tag := strings.Trim(field.Tag.Value, "`")
	for _, part := range strings.Fields(tag) {
		if strings.HasPrefix(part, "json:") {
			jsonTag := strings.Trim(strings.TrimPrefix(part, "json:"), "\"")
			// Strip options (e.g., "name,omitempty" -> "name")
			if idx := strings.Index(jsonTag, ","); idx >= 0 {
				return jsonTag[:idx]
			}
			return jsonTag
		}
	}

	return ""
}

// exprToString converts an AST expression to a string
func (g *Generator) exprToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + g.exprToString(t.X)
	case *ast.ArrayType:
		return "[]" + g.exprToString(t.Elt)
	case *ast.MapType:
		return "map[" + g.exprToString(t.Key) + "]" + g.exprToString(t.Value)
	case *ast.SelectorExpr:
		return g.exprToString(t.X) + "." + t.Sel.Name
	default:
		return fmt.Sprintf("%T", t)
	}
}

// Placeholder methods - will implement in next steps

func (g *Generator) generateRESTTypes() error {
	// TODO: Implement Phase 1
	return fmt.Errorf("generateRESTTypes not yet implemented")
}

func (g *Generator) generateServiceSetFields() error {
	// TODO: Implement Phase 2
	return fmt.Errorf("generateServiceSetFields not yet implemented")
}

func (g *Generator) generateConversionFunctions() error {
	// TODO: Implement Phase 3
	return fmt.Errorf("generateConversionFunctions not yet implemented")
}

// ensureDir creates a directory if it doesn't exist
func (g *Generator) ensureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// writeFile writes content to a file, creating parent directories as needed
func (g *Generator) writeFile(relativePath, content string) error {
	fullPath := filepath.Join(g.OutputDir, relativePath)

	if err := g.ensureDir(filepath.Dir(fullPath)); err != nil {
		return err
	}

	return os.WriteFile(fullPath, []byte(content), 0644)
}

// sortedKeys returns sorted keys from a map
func sortedKeys[K ~string, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})
	return keys
}
