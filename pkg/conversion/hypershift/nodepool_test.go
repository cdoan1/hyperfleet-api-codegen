package hypershift

import (
	"testing"

	v1alpha1 "github.com/cdoan1/hyperfleet-api-codegen/api/v1alpha1"
	hypershiftv1beta1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestToHyperShiftNodePool_Nil(t *testing.T) {
	result := ToHyperShiftNodePool(nil)
	if result != nil {
		t.Errorf("Expected nil result for nil input, got %v", result)
	}
}

func TestToHyperShiftNodePool_BasicFields(t *testing.T) {
	replicas := int32(3)
	np := &v1alpha1.NodePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-nodepool",
			Namespace: "clusters",
			Labels: map[string]string{
				"env": "test",
			},
			Annotations: map[string]string{
				"owner": "test-team",
			},
		},
		Spec: v1alpha1.NodePoolSpec{
			// Envelope fields (should NOT be copied)
			ClusterRef: v1alpha1.ClusterReference{
				Name: "test-cluster",
			},
			DisplayName: "Test NodePool",
			AutoRepair:  boolPtr(true),

			// Passthrough fields
			NodePool: v1alpha1.NodePoolSpecPassthrough{
				ClusterName: "test-cluster",
				Replicas:    &replicas,
				Platform: hypershiftv1beta1.NodePoolPlatform{
					Type: hypershiftv1beta1.AWSPlatform,
				},
				Arch: "amd64",
			},
		},
	}

	result := ToHyperShiftNodePool(np)

	// Verify metadata
	if result.Name != "test-nodepool" {
		t.Errorf("Expected Name=test-nodepool, got %s", result.Name)
	}
	if result.Namespace != "clusters" {
		t.Errorf("Expected Namespace=clusters, got %s", result.Namespace)
	}
	if result.Labels["env"] != "test" {
		t.Errorf("Expected Labels[env]=test, got %s", result.Labels["env"])
	}
	if result.Annotations["owner"] != "test-team" {
		t.Errorf("Expected Annotations[owner]=test-team, got %s", result.Annotations["owner"])
	}

	// Verify passthrough fields were copied
	if result.Spec.ClusterName != "test-cluster" {
		t.Errorf("Expected ClusterName=test-cluster, got %s", result.Spec.ClusterName)
	}
	if *result.Spec.Replicas != 3 {
		t.Errorf("Expected Replicas=3, got %d", *result.Spec.Replicas)
	}
	if result.Spec.Platform.Type != hypershiftv1beta1.AWSPlatform {
		t.Errorf("Expected Platform.Type=AWS, got %s", result.Spec.Platform.Type)
	}
	if result.Spec.Arch != "amd64" {
		t.Errorf("Expected Arch=amd64, got %s", result.Spec.Arch)
	}
}

func TestFromHyperShiftNodePool_Nil(t *testing.T) {
	result := FromHyperShiftNodePool(nil)

	// Should return empty status, not panic
	if result.State != "" {
		t.Errorf("Expected empty State for nil input, got %s", result.State)
	}
	if result.Replicas != 0 {
		t.Errorf("Expected Replicas=0 for nil input, got %d", result.Replicas)
	}
}

func TestFromHyperShiftNodePool_StatusMapping(t *testing.T) {
	replicas := int32(3)
	np := &hypershiftv1beta1.NodePool{
		Status: hypershiftv1beta1.NodePoolStatus{
			Replicas: replicas,
			Conditions: []hypershiftv1beta1.NodePoolCondition{
				{
					Type:               "Ready",
					Status:             "True",
					LastTransitionTime: metav1.Now(),
					Reason:             "AsExpected",
					Message:            "NodePool is ready",
				},
				{
					Type:               "AllNodesHealthy",
					Status:             "True",
					LastTransitionTime: metav1.Now(),
					Reason:             "NodesHealthy",
					Message:            "All nodes are healthy",
				},
			},
		},
	}

	status := FromHyperShiftNodePool(np)

	// Verify replicas were mapped
	if status.Replicas != 3 {
		t.Errorf("Expected Replicas=3, got %d", status.Replicas)
	}

	// Verify conditions were copied
	if len(status.Conditions) != 2 {
		t.Errorf("Expected 2 conditions, got %d", len(status.Conditions))
	}

	// Verify state was computed
	if status.State != "ready" {
		t.Errorf("Expected State=ready (derived from Ready=True), got %s", status.State)
	}
}

func TestComputeNodePoolState(t *testing.T) {
	tests := []struct {
		name       string
		conditions []metav1.Condition
		wantState  string
	}{
		{
			name:       "no conditions",
			conditions: []metav1.Condition{},
			wantState:  "pending",
		},
		{
			name: "ready=true",
			conditions: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionTrue},
			},
			wantState: "ready",
		},
		{
			name: "updating version",
			conditions: []metav1.Condition{
				{Type: "UpdatingVersion", Status: metav1.ConditionTrue},
			},
			wantState: "updating",
		},
		{
			name: "nodes unhealthy",
			conditions: []metav1.Condition{
				{Type: "AllNodesHealthy", Status: metav1.ConditionFalse},
			},
			wantState: "degraded",
		},
		{
			name: "ready takes precedence over updating",
			conditions: []metav1.Condition{
				{Type: "UpdatingVersion", Status: metav1.ConditionTrue},
				{Type: "Ready", Status: metav1.ConditionTrue},
			},
			wantState: "ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeNodePoolState(tt.conditions)
			if got != tt.wantState {
				t.Errorf("computeNodePoolState() = %v, want %v", got, tt.wantState)
			}
		})
	}
}
