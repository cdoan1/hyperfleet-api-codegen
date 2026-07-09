package v1alpha1

import (
	"testing"

	v1alpha1 "github.com/cdoan1/hyperfleet-api-codegen/api/v1alpha1"
	"github.com/cdoan1/hyperfleet-api-codegen/pkg/conversion"
	"github.com/cdoan1/hyperfleet-api-codegen/pkg/conversion/v1alpha1/rest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestClusterRoundTrip(t *testing.T) {
	// Create a CRD Cluster with both visible and hidden fields
	now := metav1.Now()
	crd := &v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			UID: types.UID("test-cluster-123"),
		},
		Spec: v1alpha1.ClusterSpec{
			// Visible fields
			DisplayName:         "Test Cluster",
			DeleteProtection:    boolPtr(true),
			ExpirationTimestamp: &now,
			Properties:          map[string]string{"env": "test"},
			Tags:                map[string]string{"team": "platform"},

			// Hidden fields (service-set)
			AccountID:  "acc-12345",
			CreatorARN: "arn:aws:iam::123456789012:user/test",
			InternalID: "internal-abc",
		},
		Status: v1alpha1.ClusterStatus{
			State:       "Ready",
			Conditions:  []metav1.Condition{},
			Version:     "4.14.0",
			APIEndpoint: "https://api.test.example.com:6443",
			ConsoleURL:  "https://console.test.example.com",
		},
	}

	// CRD → REST (should strip hidden fields)
	restCluster := ProjectCluster(crd)

	// Verify visible fields are present
	if restCluster.Spec.DisplayName != "Test Cluster" {
		t.Errorf("Expected DisplayName 'Test Cluster', got %q", restCluster.Spec.DisplayName)
	}
	if restCluster.Spec.DeleteProtection == nil || !*restCluster.Spec.DeleteProtection {
		t.Error("Expected DeleteProtection true")
	}
	if restCluster.Spec.ExpirationTimestamp == nil {
		t.Error("Expected ExpirationTimestamp to be set")
	}
	if len(restCluster.Spec.Properties) != 1 || restCluster.Spec.Properties["env"] != "test" {
		t.Errorf("Expected Properties map with env=test, got %v", restCluster.Spec.Properties)
	}
	if len(restCluster.Spec.Tags) != 1 || restCluster.Spec.Tags["team"] != "platform" {
		t.Errorf("Expected Tags map with team=platform, got %v", restCluster.Spec.Tags)
	}

	// Verify status fields
	if restCluster.Status.State != "Ready" {
		t.Errorf("Expected State 'Ready', got %q", restCluster.Status.State)
	}
	if restCluster.Status.Version != "4.14.0" {
		t.Errorf("Expected Version '4.14.0', got %q", restCluster.Status.Version)
	}

	// REST → CRD with enrichment (should add service-set fields back)
	enrichment := &conversion.ServiceSetFields{
		AccountID:  "acc-enriched",
		CreatorARN: "arn:aws:iam::999:user/enriched",
		InternalID: "internal-enriched",
	}
	crdSpec := UnprojectCluster(&restCluster.Spec, enrichment)

	// Verify visible fields preserved
	if crdSpec.DisplayName != "Test Cluster" {
		t.Errorf("Expected DisplayName 'Test Cluster', got %q", crdSpec.DisplayName)
	}
	if crdSpec.DeleteProtection == nil || !*crdSpec.DeleteProtection {
		t.Error("Expected DeleteProtection true")
	}

	// Verify service-set fields were injected from enrichment
	if crdSpec.AccountID != "acc-enriched" {
		t.Errorf("Expected AccountID 'acc-enriched', got %q", crdSpec.AccountID)
	}
	if crdSpec.CreatorARN != "arn:aws:iam::999:user/enriched" {
		t.Errorf("Expected CreatorARN from enrichment, got %q", crdSpec.CreatorARN)
	}
	if crdSpec.InternalID != "internal-enriched" {
		t.Errorf("Expected InternalID from enrichment, got %q", crdSpec.InternalID)
	}
}

func TestNodePoolRoundTrip(t *testing.T) {
	// Create a CRD NodePool with both visible and hidden fields
	crd := &v1alpha1.NodePool{
		ObjectMeta: metav1.ObjectMeta{
			UID: types.UID("test-nodepool-456"),
		},
		Spec: v1alpha1.NodePoolSpec{
			ClusterRef: v1alpha1.ClusterReference{
				Name:      "test-cluster",
				Namespace: "default",
			},
			DisplayName: "Worker Pool",
			AutoRepair:  boolPtr(true),
			Labels:      map[string]string{"role": "worker"},

			// Hidden fields (service-set)
			AccountID:      "acc-67890",
			InternalPoolID: "pool-xyz",
		},
		Status: v1alpha1.NodePoolStatus{
			State:         "Ready",
			Conditions:    []metav1.Condition{},
			Replicas:      3,
			ReadyReplicas: 3,
		},
	}

	// CRD → REST
	restNodePool := ProjectNodePool(crd)

	// Verify visible fields
	if restNodePool.Spec.DisplayName != "Worker Pool" {
		t.Errorf("Expected DisplayName 'Worker Pool', got %q", restNodePool.Spec.DisplayName)
	}
	if restNodePool.Spec.ClusterRef.Name != "test-cluster" {
		t.Errorf("Expected ClusterRef.Name 'test-cluster', got %q", restNodePool.Spec.ClusterRef.Name)
	}
	if restNodePool.Spec.AutoRepair == nil || !*restNodePool.Spec.AutoRepair {
		t.Error("Expected AutoRepair true")
	}

	// Verify status
	if restNodePool.Status.State != "Ready" {
		t.Errorf("Expected State 'Ready', got %q", restNodePool.Status.State)
	}
	if restNodePool.Status.Replicas != 3 {
		t.Errorf("Expected Replicas 3, got %d", restNodePool.Status.Replicas)
	}

	// REST → CRD with enrichment
	enrichment := &conversion.ServiceSetFields{
		AccountID:      "acc-new",
		InternalPoolID: "pool-new",
	}
	crdSpec := UnprojectNodePool(&restNodePool.Spec, enrichment)

	// Verify visible fields preserved
	if crdSpec.DisplayName != "Worker Pool" {
		t.Errorf("Expected DisplayName 'Worker Pool', got %q", crdSpec.DisplayName)
	}
	if crdSpec.ClusterRef.Name != "test-cluster" {
		t.Errorf("Expected ClusterRef.Name 'test-cluster', got %q", crdSpec.ClusterRef.Name)
	}

	// Verify service-set fields injected
	if crdSpec.AccountID != "acc-new" {
		t.Errorf("Expected AccountID 'acc-new', got %q", crdSpec.AccountID)
	}
	if crdSpec.InternalPoolID != "pool-new" {
		t.Errorf("Expected InternalPoolID 'pool-new', got %q", crdSpec.InternalPoolID)
	}
}

func TestProjectStripsHiddenFields(t *testing.T) {
	// This test verifies that hidden fields don't appear in REST types
	// It's a compile-time guarantee - if REST types had hidden fields,
	// the code wouldn't compile

	crd := &v1alpha1.Cluster{
		Spec: v1alpha1.ClusterSpec{
			DisplayName: "test",
			AccountID:   "should-not-appear", // Hidden field
		},
	}

	rest := ProjectCluster(crd)

	// Compile-time check: rest.Spec.AccountID doesn't exist
	// If it did, this test file wouldn't compile
	_ = rest.Spec.DisplayName // This compiles (visible)
	// _ = rest.Spec.AccountID // This would be a compile error (hidden)
}

func TestUnprojectNilEnrichment(t *testing.T) {
	// Verify that Unproject works when enrichment is nil
	restSpec := &rest.ClusterSpec{
		DisplayName: "test",
	}

	crdSpec := UnprojectCluster(restSpec, nil)

	if crdSpec.DisplayName != "test" {
		t.Errorf("Expected DisplayName 'test', got %q", crdSpec.DisplayName)
	}

	// Service-set fields should be zero values when enrichment is nil
	if crdSpec.AccountID != "" {
		t.Errorf("Expected AccountID to be empty, got %q", crdSpec.AccountID)
	}
	if crdSpec.CreatorARN != "" {
		t.Errorf("Expected CreatorARN to be empty, got %q", crdSpec.CreatorARN)
	}
}

func TestProjectNilPointer(t *testing.T) {
	// Verify that Project functions handle nil pointers gracefully
	var nilCluster *v1alpha1.Cluster = nil
	restCluster := ProjectCluster(nilCluster)
	if restCluster != nil {
		t.Error("Expected nil result from ProjectCluster(nil)")
	}

	var nilNodePool *v1alpha1.NodePool = nil
	restNodePool := ProjectNodePool(nilNodePool)
	if restNodePool != nil {
		t.Error("Expected nil result from ProjectNodePool(nil)")
	}
}

func TestUnprojectNilSpec(t *testing.T) {
	// Verify that Unproject functions handle nil specs gracefully
	crdSpec := UnprojectCluster(nil, nil)
	if crdSpec != nil {
		t.Error("Expected nil result from UnprojectCluster(nil, nil)")
	}

	npSpec := UnprojectNodePool(nil, nil)
	if npSpec != nil {
		t.Error("Expected nil result from UnprojectNodePool(nil, nil)")
	}
}

// Helper functions
func boolPtr(b bool) *bool {
	return &b
}
