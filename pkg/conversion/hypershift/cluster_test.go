package hypershift

import (
	"testing"

	v1alpha1 "github.com/cdoan1/hyperfleet-api-codegen/api/v1alpha1"
	configv1 "github.com/openshift/api/config/v1"
	hypershiftv1beta1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestToHyperShiftHostedCluster_Nil(t *testing.T) {
	result := ToHyperShiftHostedCluster(nil)
	if result != nil {
		t.Errorf("Expected nil result for nil input, got %v", result)
	}
}

func TestToHyperShiftHostedCluster_BasicFields(t *testing.T) {
	cluster := &v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "clusters",
			Labels: map[string]string{
				"env": "test",
			},
			Annotations: map[string]string{
				"owner": "test-team",
			},
		},
		Spec: v1alpha1.ClusterSpec{
			// Envelope fields (should NOT be copied)
			DisplayName:         "Test Cluster",
			DeleteProtection:    boolPtr(true),
			ExpirationTimestamp: &metav1.Time{Time: metav1.Now().Time},
			Properties: map[string]string{
				"key": "value",
			},
			// Passthrough fields
			HostedCluster: v1alpha1.HostedClusterSpecPassthrough{
				ClusterID: "test-cluster-id",
				InfraID:   "test-infra-id",
				FIPS:      true,
				Channel:   "stable",
				Release: hypershiftv1beta1.Release{
					Image: "quay.io/openshift-release-dev/ocp-release:4.14.0-x86_64",
				},
				PullSecret: corev1.LocalObjectReference{
					Name: "pull-secret",
				},
				SSHKey: corev1.LocalObjectReference{
					Name: "ssh-key",
				},
			},
		},
	}

	result := ToHyperShiftHostedCluster(cluster)

	// Verify metadata
	if result.Name != "test-cluster" {
		t.Errorf("Expected Name=test-cluster, got %s", result.Name)
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
	if result.Spec.ClusterID != "test-cluster-id" {
		t.Errorf("Expected ClusterID=test-cluster-id, got %s", result.Spec.ClusterID)
	}
	if result.Spec.InfraID != "test-infra-id" {
		t.Errorf("Expected InfraID=test-infra-id, got %s", result.Spec.InfraID)
	}
	if !result.Spec.FIPS {
		t.Error("Expected FIPS=true")
	}
	if result.Spec.Channel != "stable" {
		t.Errorf("Expected Channel=stable, got %s", result.Spec.Channel)
	}
	if result.Spec.PullSecret.Name != "pull-secret" {
		t.Errorf("Expected PullSecret.Name=pull-secret, got %s", result.Spec.PullSecret.Name)
	}
	if result.Spec.SSHKey.Name != "ssh-key" {
		t.Errorf("Expected SSHKey.Name=ssh-key, got %s", result.Spec.SSHKey.Name)
	}
}

func TestFromHyperShiftHostedCluster_Nil(t *testing.T) {
	result := FromHyperShiftHostedCluster(nil)

	// Should return empty status, not panic
	if result.State != "" {
		t.Errorf("Expected empty State for nil input, got %s", result.State)
	}
}

func TestFromHyperShiftHostedCluster_StatusMapping(t *testing.T) {
	hc := &hypershiftv1beta1.HostedCluster{
		Status: hypershiftv1beta1.HostedClusterStatus{
			Version: &hypershiftv1beta1.ClusterVersionStatus{
				Desired: configv1.Release{
					Image:   "quay.io/openshift-release-dev/ocp-release:4.14.0-x86_64",
					Version: "4.14.0-x86_64",
				},
			},
			Conditions: []metav1.Condition{
				{
					Type:               "Available",
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Reason:             "AsExpected",
					Message:            "Cluster is available",
				},
				{
					Type:               "Progressing",
					Status:             metav1.ConditionFalse,
					LastTransitionTime: metav1.Now(),
					Reason:             "AsExpected",
					Message:            "No updates in progress",
				},
			},
		},
	}

	status := FromHyperShiftHostedCluster(hc)

	// Verify version was mapped
	if status.Version != "4.14.0-x86_64" {
		t.Errorf("Expected Version=4.14.0-x86_64, got %s", status.Version)
	}

	// Verify conditions were copied
	if len(status.Conditions) != 2 {
		t.Errorf("Expected 2 conditions, got %d", len(status.Conditions))
	}

	// Verify state was computed
	if status.State != "ready" {
		t.Errorf("Expected State=ready (derived from Available=True), got %s", status.State)
	}
}

func TestComputeClusterState(t *testing.T) {
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
			name: "available=true",
			conditions: []metav1.Condition{
				{Type: "Available", Status: metav1.ConditionTrue},
			},
			wantState: "ready",
		},
		{
			name: "progressing=true",
			conditions: []metav1.Condition{
				{Type: "Progressing", Status: metav1.ConditionTrue},
			},
			wantState: "provisioning",
		},
		{
			name: "degraded=true",
			conditions: []metav1.Condition{
				{Type: "Degraded", Status: metav1.ConditionTrue},
			},
			wantState: "degraded",
		},
		{
			name: "available=true takes precedence",
			conditions: []metav1.Condition{
				{Type: "Progressing", Status: metav1.ConditionTrue},
				{Type: "Available", Status: metav1.ConditionTrue},
			},
			wantState: "ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeClusterState(tt.conditions)
			if got != tt.wantState {
				t.Errorf("computeClusterState() = %v, want %v", got, tt.wantState)
			}
		})
	}
}

// Helper function
func boolPtr(b bool) *bool {
	return &b
}
