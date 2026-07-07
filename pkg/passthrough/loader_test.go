package passthrough

import (
	"testing"

	"github.com/openshift-online/hyperfleet-api-codegen/pkg/markers"
)

func TestLoadHyperShiftTypes(t *testing.T) {
	gen := NewGenerator(
		"github.com/openshift/hypershift/api/hypershift/v1beta1",
		[]string{"HostedClusterSpec"},
		make(markers.FieldRegistry),
	)

	if err := gen.LoadSourcePackage(); err != nil {
		t.Fatalf("Failed to load package: %v", err)
	}

	if gen.pkg == nil {
		t.Fatal("Package was not loaded")
	}

	t.Logf("Loaded package: %s", gen.pkg.Path())
}

func TestGenerateTypeDef(t *testing.T) {
	gen := NewGenerator(
		"github.com/openshift/hypershift/api/hypershift/v1beta1",
		[]string{"HostedClusterSpec"},
		make(markers.FieldRegistry),
	)

	if err := gen.LoadSourcePackage(); err != nil {
		t.Fatalf("Failed to load package: %v", err)
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
