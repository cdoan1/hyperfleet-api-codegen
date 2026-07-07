package passthrough

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
)

// LoadSourceFiles loads and parses Go source files from a directory
func (g *Generator) LoadSourceFiles(sourceDir string) error {
	fset := token.NewFileSet()

	// Parse all Go files in the directory
	//nolint:staticcheck // ParseDir is sufficient for our use case of parsing single directories
	pkgs, err := parser.ParseDir(fset, sourceDir, func(fi os.FileInfo) bool {
		// Skip test files and generated files
		name := fi.Name()
		return !strings.HasSuffix(name, "_test.go") &&
			!strings.HasPrefix(name, "zz_generated")
	}, parser.ParseComments)

	if err != nil {
		return fmt.Errorf("parsing directory %s: %w", sourceDir, err)
	}

	if len(pkgs) == 0 {
		return fmt.Errorf("no packages found in %s", sourceDir)
	}

	// Store parsed files
	g.parsedFiles = make(map[string]*ast.File)
	for _, pkg := range pkgs {
		for filename, file := range pkg.Files {
			g.parsedFiles[filename] = file
		}
	}

	return nil
}

// GenerateTypeDef creates a passthrough type definition for a source type
func (g *Generator) GenerateTypeDef(typeName string) (*TypeDef, error) {
	// Find the type definition across all parsed files
	var typeSpec *ast.TypeSpec
	for _, file := range g.parsedFiles {
		ast.Inspect(file, func(n ast.Node) bool {
			if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == typeName {
				typeSpec = ts
				return false
			}
			return true
		})
		if typeSpec != nil {
			break
		}
	}

	if typeSpec == nil {
		return nil, fmt.Errorf("type %s not found in parsed files", typeName)
	}

	// Ensure it's a struct type
	structType, ok := typeSpec.Type.(*ast.StructType)
	if !ok {
		return nil, fmt.Errorf("type %s is not a struct", typeName)
	}

	typeDef := &TypeDef{
		Name:       typeName + "Passthrough",
		SourceName: typeName,
		Doc:        fmt.Sprintf("%s mirrors %s from upstream", typeName+"Passthrough", typeName),
		Fields:     make([]FieldDef, 0),
	}

	// Process each field
	for _, field := range structType.Fields.List {
		// Skip fields without names (embedded types)
		if len(field.Names) == 0 {
			continue
		}

		for _, name := range field.Names {
			// Skip unexported fields
			if !name.IsExported() {
				continue
			}

			fieldDef := g.createFieldDef(name.Name, field)
			typeDef.Fields = append(typeDef.Fields, fieldDef)
		}
	}

	return typeDef, nil
}

// createFieldDef creates a field definition with appropriate markers
func (g *Generator) createFieldDef(fieldName string, field *ast.Field) FieldDef {
	fieldDef := FieldDef{
		Name: fieldName,
		Type: g.typeToString(field.Type),
	}

	// Extract JSON tag
	if field.Tag != nil {
		tag := strings.Trim(field.Tag.Value, "`")
		if jsonTag := parseStructTag(tag, "json"); jsonTag != "" {
			fieldDef.JSONTag = jsonTag
		}
	}

	// Extract documentation (first line only, collapsed to single line)
	if field.Doc != nil {
		doc := strings.TrimSpace(field.Doc.Text())
		// Take only first line and collapse to single line
		lines := strings.Split(doc, "\n")
		if len(lines) > 0 {
			fieldDef.Doc = strings.TrimSpace(lines[0])
		}
	}

	// Get markers for this field
	fieldDef.Markers = g.getMarkersForField(fieldName)

	return fieldDef
}

// typeToString converts an AST type expression to a string
func (g *Generator) typeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + g.typeToString(t.X)
	case *ast.ArrayType:
		return "[]" + g.typeToString(t.Elt)
	case *ast.MapType:
		return "map[" + g.typeToString(t.Key) + "]" + g.typeToString(t.Value)
	case *ast.SelectorExpr:
		return g.typeToString(t.X) + "." + t.Sel.Name
	default:
		return "interface{}"
	}
}

// getMarkersForField returns markers for a field, from registry or defaults
func (g *Generator) getMarkersForField(fieldName string) []string {
	// Check if we have existing markers in the registry
	if meta, found := g.Registry[fieldName]; found {
		var markers []string

		// Add visibility marker if hidden
		if meta.Hidden {
			markers = append(markers, "+k8s:openapi-gen=false")
		}

		// Add write mode marker
		if meta.WriteMode != "" {
			markers = append(markers, fmt.Sprintf("+hyperfleet:write-mode=%s", meta.WriteMode))
		}

		// Add feature gate marker
		if meta.FeatureGate != "" {
			markers = append(markers, fmt.Sprintf("+openshift:enable:FeatureGate=%s", meta.FeatureGate))
		}

		return markers
	}

	// Apply safe defaults for new fields
	return []string{
		"+k8s:openapi-gen=false",
		"+hyperfleet:write-mode=service-set",
	}
}

// parseStructTag extracts a specific tag value from struct tag string
func parseStructTag(tag, key string) string {
	// Simple tag parser - handles: `json:"name,omitempty" yaml:"name"`
	parts := strings.Fields(tag)
	prefix := key + `:"`

	for _, part := range parts {
		if strings.HasPrefix(part, prefix) {
			value := strings.TrimPrefix(part, prefix)
			value = strings.TrimSuffix(value, `"`)
			return value
		}
	}

	return ""
}
