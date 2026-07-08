package passthrough

import (
	"testing"

	"github.com/cdoan1/hyperfleet-api-codegen/pkg/markers"
)

func TestLoadHyperShiftTypes(t *testing.T) {
	// Create generator from import path (resolves via go.mod)
	gen, err := NewGeneratorFromImportPath(
		"github.com/openshift/hypershift/api/hypershift/v1beta1",
		[]string{"HostedClusterSpec"},
		make(markers.FieldRegistry),
	)
	if err != nil {
		t.Fatalf("Failed to create generator: %v", err)
	}

	if err := gen.LoadSourceFiles(gen.SourceDir); err != nil {
		t.Fatalf("Failed to load source files: %v", err)
	}

	if len(gen.parsedFiles) == 0 {
		t.Fatal("No files were loaded")
	}

	t.Logf("Loaded %d files from %s", len(gen.parsedFiles), gen.SourceDir)
}

func TestGenerateTypeDef(t *testing.T) {
	// Create generator from import path (resolves via go.mod)
	gen, err := NewGeneratorFromImportPath(
		"github.com/openshift/hypershift/api/hypershift/v1beta1",
		[]string{"HostedClusterSpec"},
		make(markers.FieldRegistry),
	)
	if err != nil {
		t.Fatalf("Failed to create generator: %v", err)
	}

	if err := gen.LoadSourceFiles(gen.SourceDir); err != nil {
		t.Fatalf("Failed to load source files: %v", err)
	}

	typeDef, err := gen.GenerateTypeDef("HostedClusterSpec")
	if err != nil {
		t.Fatalf("Failed to generate type def: %v", err)
	}

	if typeDef.Name != "HostedClusterSpecPassthrough" {
		t.Errorf("Expected name HostedClusterSpecPassthrough, got %s", typeDef.Name)
	}

	if len(typeDef.Fields) == 0 {
		t.Error("Expected some fields, got none")
	}

	t.Logf("Generated %d fields for %s", len(typeDef.Fields), typeDef.Name)
	for i, field := range typeDef.Fields {
		if i < 5 { // Show first 5 fields
			t.Logf("  Field %d: %s %s `json:\"%s\"`", i, field.Name, field.Type, field.JSONTag)
			t.Logf("    Markers: %v", field.Markers)
		}
	}
}
