package passthrough

import (
	"fmt"
	"go/types"

	"golang.org/x/tools/go/packages"
)

// LoadSourcePackage loads type information from the source package
func (g *Generator) LoadSourcePackage() error {
	cfg := &packages.Config{
		Mode: packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax | packages.NeedName,
	}

	pkgs, err := packages.Load(cfg, g.SourcePackage)
	if err != nil {
		return fmt.Errorf("loading package %s: %w", g.SourcePackage, err)
	}

	if len(pkgs) == 0 {
		return fmt.Errorf("no packages found for %s", g.SourcePackage)
	}

	if len(pkgs[0].Errors) > 0 {
		return fmt.Errorf("errors loading package: %v", pkgs[0].Errors)
	}

	g.pkg = pkgs[0].Types
	g.typeInfo = pkgs[0].TypesInfo

	return nil
}

// GenerateTypeDef creates a passthrough type definition for a source type
func (g *Generator) GenerateTypeDef(typeName string) (*TypeDef, error) {
	// Find the type in the package
	obj := g.pkg.Scope().Lookup(typeName)
	if obj == nil {
		return nil, fmt.Errorf("type %s not found in package %s", typeName, g.SourcePackage)
	}

	// Get the underlying struct type
	named, ok := obj.Type().(*types.Named)
	if !ok {
		return nil, fmt.Errorf("type %s is not a named type", typeName)
	}

	structType, ok := named.Underlying().(*types.Struct)
	if !ok {
		return nil, fmt.Errorf("type %s is not a struct", typeName)
	}

	typeDef := &TypeDef{
		Name:       typeName + "Passthrough",
		SourceName: typeName,
		Doc:        fmt.Sprintf("%s mirrors %s from %s", typeName+"Passthrough", typeName, g.SourcePackage),
		Fields:     make([]FieldDef, 0),
	}

	// Process each field
	for i := 0; i < structType.NumFields(); i++ {
		field := structType.Field(i)
		tag := structType.Tag(i)

		// Skip unexported fields
		if !field.Exported() {
			continue
		}

		fieldDef := g.createFieldDef(field, tag)
		typeDef.Fields = append(typeDef.Fields, fieldDef)
	}

	return typeDef, nil
}

// createFieldDef creates a field definition with appropriate markers
func (g *Generator) createFieldDef(field *types.Var, tag string) FieldDef {
	fieldDef := FieldDef{
		Name: field.Name(),
		Type: types.TypeString(field.Type(), nil),
		Doc:  fmt.Sprintf("%s field from upstream", field.Name()),
	}

	// Extract JSON tag
	if tag != "" {
		fieldDef.JSONTag = tag
	}

	// Determine markers based on field path and registry
	// For now, apply safe defaults
	fieldDef.Markers = g.getMarkersForField(field.Name())

	return fieldDef
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
