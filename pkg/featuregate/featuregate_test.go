package featuregate

import (
	"testing"
)

func TestFeatureStageHierarchy(t *testing.T) {
	tests := []struct {
		name        string
		featureSet  FeatureSet
		stage       FeatureStage
		shouldInclude bool
	}{
		{"Default includes GA", Default, GA, true},
		{"Default excludes TechPreview", Default, TechPreview, false},
		{"Default excludes DevPreview", Default, DevPreview, false},

		{"TechPreview includes GA", TechPreviewNoUpgrade, GA, true},
		{"TechPreview includes TechPreview", TechPreviewNoUpgrade, TechPreview, true},
		{"TechPreview excludes DevPreview", TechPreviewNoUpgrade, DevPreview, false},

		{"DevPreview includes GA", DevPreviewNoUpgrade, GA, true},
		{"DevPreview includes TechPreview", DevPreviewNoUpgrade, TechPreview, true},
		{"DevPreview includes DevPreview", DevPreviewNoUpgrade, DevPreview, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.featureSet.Includes(tt.stage)
			if got != tt.shouldInclude {
				t.Errorf("FeatureSet.Includes() = %v, want %v", got, tt.shouldInclude)
			}
		})
	}
}

func TestIsGateEnabled(t *testing.T) {
	tests := []struct {
		name       string
		gate       string
		featureSet FeatureSet
		want       bool
	}{
		{"GA gate in Default", "HyperFleetEtcdConfig", Default, true},
		{"GA gate in TechPreview", "HyperFleetEtcdConfig", TechPreviewNoUpgrade, true},
		{"GA gate in DevPreview", "HyperFleetEtcdConfig", DevPreviewNoUpgrade, true},

		{"TechPreview gate in Default", "HyperFleetAutoScaling", Default, false},
		{"TechPreview gate in TechPreview", "HyperFleetAutoScaling", TechPreviewNoUpgrade, true},
		{"TechPreview gate in DevPreview", "HyperFleetAutoScaling", DevPreviewNoUpgrade, true},

		{"DevPreview gate in Default", "HyperFleetCustomDNS", Default, false},
		{"DevPreview gate in TechPreview", "HyperFleetCustomDNS", TechPreviewNoUpgrade, false},
		{"DevPreview gate in DevPreview", "HyperFleetCustomDNS", DevPreviewNoUpgrade, true},

		{"Unknown gate is disabled", "NonExistentGate", DevPreviewNoUpgrade, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsGateEnabled(tt.gate, tt.featureSet)
			if got != tt.want {
				t.Errorf("IsGateEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGatesForFeatureSet(t *testing.T) {
	tests := []struct {
		name       string
		featureSet FeatureSet
		wantCount  int
	}{
		{"Default has 1 gate (GA only)", Default, 1},
		{"TechPreview has 3 gates (GA + TechPreview)", TechPreviewNoUpgrade, 3},
		{"DevPreview has 4 gates (GA + TechPreview + DevPreview)", DevPreviewNoUpgrade, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gates := GatesForFeatureSet(tt.featureSet)
			if len(gates) != tt.wantCount {
				t.Errorf("GatesForFeatureSet() returned %d gates, want %d", len(gates), tt.wantCount)
			}
		})
	}
}

func TestFilterCRDFields(t *testing.T) {
	// This test verifies that feature gates properly filter fields
	// Note: Actual field counts depend on the current registry

	defaultFields := FilterCRDFields(Default)
	techPreviewFields := FilterCRDFields(TechPreviewNoUpgrade)
	devPreviewFields := FilterCRDFields(DevPreviewNoUpgrade)

	// DevPreview should have >= TechPreview should have >= Default
	if len(defaultFields) > len(techPreviewFields) {
		t.Errorf("Default has more fields (%d) than TechPreview (%d)",
			len(defaultFields), len(techPreviewFields))
	}

	if len(techPreviewFields) > len(devPreviewFields) {
		t.Errorf("TechPreview has more fields (%d) than DevPreview (%d)",
			len(techPreviewFields), len(devPreviewFields))
	}

	t.Logf("Default: %d fields", len(defaultFields))
	t.Logf("TechPreview: %d fields", len(techPreviewFields))
	t.Logf("DevPreview: %d fields", len(devPreviewFields))
}
