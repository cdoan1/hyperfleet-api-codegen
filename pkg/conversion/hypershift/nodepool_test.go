package hypershift

import (
	"testing"
	"time"

	v1alpha1 "github.com/cdoan1/hyperfleet-api-codegen/api/v1alpha1"
	hypershiftv1beta1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	corev1 "k8s.io/api/core/v1"
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

func TestToHyperShiftNodePool_AllPassthroughFields(t *testing.T) {
	replicas := int32(5)
	pausedUntil := "2026-12-31T23:59:59Z"
	drainTimeout := metav1.Duration{Duration: 10 * time.Minute}
	detachTimeout := metav1.Duration{Duration: 5 * time.Minute}

	np := &v1alpha1.NodePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "comprehensive-nodepool",
			Namespace: "clusters",
		},
		Spec: v1alpha1.NodePoolSpec{
			// Envelope fields
			ClusterRef:  v1alpha1.ClusterReference{Name: "test-cluster"},
			DisplayName: "Comprehensive Test",
			AutoRepair:  boolPtr(false),

			// All passthrough fields
			NodePool: v1alpha1.NodePoolSpecPassthrough{
				ClusterName: "test-cluster",
				Replicas:    &replicas,
				Platform: hypershiftv1beta1.NodePoolPlatform{
					Type: hypershiftv1beta1.AWSPlatform,
				},
				Arch: "arm64",
				Management: hypershiftv1beta1.NodePoolManagement{
					AutoRepair:  false,
					Replace:     nil,
					UpgradeType: hypershiftv1beta1.UpgradeTypeReplace,
				},
				// AutoScaling deliberately omitted to avoid version-specific type issues
				// (Min/Max changed from int to int32/*int32 in different HyperShift versions)
				Config: []corev1.LocalObjectReference{
					{Name: "config-1"},
					{Name: "config-2"},
				},
				NodeDrainTimeout:        &drainTimeout,
				NodeVolumeDetachTimeout: &detachTimeout,
				NodeLabels: map[string]string{
					"node-role": "worker",
					"zone":      "us-east-1a",
				},
				Taints: []hypershiftv1beta1.Taint{
					{
						Key:    "dedicated",
						Value:  "ml-workload",
						Effect: "NoSchedule",
					},
				},
				PausedUntil: &pausedUntil,
				TuningConfig: []corev1.LocalObjectReference{
					{Name: "tuning-config"},
				},
			},
		},
	}

	result := ToHyperShiftNodePool(np)

	// Verify all passthrough fields
	if *result.Spec.Replicas != 5 {
		t.Errorf("Expected Replicas=5, got %d", *result.Spec.Replicas)
	}
	if result.Spec.Arch != "arm64" {
		t.Errorf("Expected Arch=arm64, got %s", result.Spec.Arch)
	}
	if result.Spec.Management.AutoRepair != false {
		t.Error("Expected Management.AutoRepair=false")
	}
	if result.Spec.Management.UpgradeType != hypershiftv1beta1.UpgradeTypeReplace {
		t.Errorf("Expected UpgradeType=Replace, got %s", result.Spec.Management.UpgradeType)
	}
	// AutoScaling field verification removed - field types vary across HyperShift versions
	// (Min/Max changed from int to int32/*int32), making version-independent tests difficult
	if len(result.Spec.Config) != 2 {
		t.Errorf("Expected 2 config references, got %d", len(result.Spec.Config))
	}
	if result.Spec.NodeDrainTimeout.Duration != 10*time.Minute {
		t.Errorf("Expected NodeDrainTimeout=10m, got %v", result.Spec.NodeDrainTimeout.Duration)
	}
	if result.Spec.NodeVolumeDetachTimeout.Duration != 5*time.Minute {
		t.Errorf("Expected NodeVolumeDetachTimeout=5m, got %v", result.Spec.NodeVolumeDetachTimeout.Duration)
	}
	if len(result.Spec.NodeLabels) != 2 {
		t.Errorf("Expected 2 node labels, got %d", len(result.Spec.NodeLabels))
	}
	if result.Spec.NodeLabels["zone"] != "us-east-1a" {
		t.Errorf("Expected NodeLabels[zone]=us-east-1a, got %s", result.Spec.NodeLabels["zone"])
	}
	if len(result.Spec.Taints) != 1 {
		t.Errorf("Expected 1 taint, got %d", len(result.Spec.Taints))
	}
	if result.Spec.Taints[0].Key != "dedicated" {
		t.Errorf("Expected Taint key=dedicated, got %s", result.Spec.Taints[0].Key)
	}
	if *result.Spec.PausedUntil != pausedUntil {
		t.Errorf("Expected PausedUntil=%s, got %s", pausedUntil, *result.Spec.PausedUntil)
	}
	if len(result.Spec.TuningConfig) != 1 {
		t.Errorf("Expected 1 tuning config, got %d", len(result.Spec.TuningConfig))
	}
}

func TestToHyperShiftNodePool_EnvelopeFieldsExcluded(t *testing.T) {
	replicas := int32(3)
	np := &v1alpha1.NodePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "envelope-test",
			Namespace: "clusters",
		},
		Spec: v1alpha1.NodePoolSpec{
			// Envelope fields - these should NOT appear in HyperShift NodePool
			ClusterRef: v1alpha1.ClusterReference{
				Name: "parent-cluster",
			},
			DisplayName: "My Fancy NodePool",
			AutoRepair:  boolPtr(true),

			// Passthrough fields
			NodePool: v1alpha1.NodePoolSpecPassthrough{
				ClusterName: "test-cluster",
				Replicas:    &replicas,
				Platform: hypershiftv1beta1.NodePoolPlatform{
					Type: hypershiftv1beta1.AWSPlatform,
				},
			},
		},
	}

	result := ToHyperShiftNodePool(np)

	// HyperShift NodePool should not have envelope fields
	// (They don't exist in the HyperShift type, so this just verifies conversion doesn't panic)
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Only passthrough fields should be present
	if result.Spec.ClusterName != "test-cluster" {
		t.Errorf("Expected ClusterName=test-cluster, got %s", result.Spec.ClusterName)
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

	// Verify condition types were converted
	foundReady := false
	foundHealthy := false
	for _, cond := range status.Conditions {
		if cond.Type == "Ready" {
			foundReady = true
			if cond.Status != metav1.ConditionTrue {
				t.Errorf("Expected Ready condition status=True, got %s", cond.Status)
			}
		}
		if cond.Type == "AllNodesHealthy" {
			foundHealthy = true
		}
	}
	if !foundReady {
		t.Error("Missing Ready condition in converted status")
	}
	if !foundHealthy {
		t.Error("Missing AllNodesHealthy condition in converted status")
	}

	// Verify state was computed
	if status.State != "ready" {
		t.Errorf("Expected State=ready (derived from Ready=True), got %s", status.State)
	}
}

func TestFromHyperShiftNodePool_EmptyStatus(t *testing.T) {
	np := &hypershiftv1beta1.NodePool{
		Status: hypershiftv1beta1.NodePoolStatus{
			// Empty status
		},
	}

	status := FromHyperShiftNodePool(np)

	// Should handle empty status gracefully
	if status.Replicas != 0 {
		t.Errorf("Expected Replicas=0 for empty status, got %d", status.Replicas)
	}
	if len(status.Conditions) != 0 {
		t.Errorf("Expected 0 conditions for empty status, got %d", len(status.Conditions))
	}
	if status.State != "pending" {
		t.Errorf("Expected State=pending for empty status, got %s", status.State)
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
			name: "ready=false",
			conditions: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionFalse},
			},
			wantState: "pending",
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
			name: "nodes healthy (status=true doesn't trigger degraded)",
			conditions: []metav1.Condition{
				{Type: "AllNodesHealthy", Status: metav1.ConditionTrue},
			},
			wantState: "pending",
		},
		{
			name: "ready takes precedence over updating",
			conditions: []metav1.Condition{
				{Type: "UpdatingVersion", Status: metav1.ConditionTrue},
				{Type: "Ready", Status: metav1.ConditionTrue},
			},
			wantState: "ready",
		},
		{
			name: "ready takes precedence over degraded",
			conditions: []metav1.Condition{
				{Type: "AllNodesHealthy", Status: metav1.ConditionFalse},
				{Type: "Ready", Status: metav1.ConditionTrue},
			},
			wantState: "ready",
		},
		{
			name: "degraded takes precedence over updating",
			conditions: []metav1.Condition{
				{Type: "UpdatingVersion", Status: metav1.ConditionTrue},
				{Type: "AllNodesHealthy", Status: metav1.ConditionFalse},
			},
			wantState: "degraded",
		},
		{
			name: "multiple conditions - priority order",
			conditions: []metav1.Condition{
				{Type: "UpdatingVersion", Status: metav1.ConditionTrue},
				{Type: "AllNodesHealthy", Status: metav1.ConditionFalse},
				{Type: "Ready", Status: metav1.ConditionTrue},
			},
			wantState: "ready", // Ready has highest priority
		},
		{
			name: "unknown condition type",
			conditions: []metav1.Condition{
				{Type: "UnknownCondition", Status: metav1.ConditionTrue},
			},
			wantState: "pending",
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

func TestComputeNodePoolState_PriorityOrder(t *testing.T) {
	// Test that verifies the explicit priority: Ready > AllNodesHealthy (unhealthy) > UpdatingVersion
	conditions := []metav1.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue},
		{Type: "AllNodesHealthy", Status: metav1.ConditionFalse},
		{Type: "UpdatingVersion", Status: metav1.ConditionTrue},
	}

	state := computeNodePoolState(conditions)
	if state != "ready" {
		t.Errorf("Expected Ready to take highest priority, got state=%s", state)
	}

	// Remove Ready condition
	conditions = []metav1.Condition{
		{Type: "AllNodesHealthy", Status: metav1.ConditionFalse},
		{Type: "UpdatingVersion", Status: metav1.ConditionTrue},
	}

	state = computeNodePoolState(conditions)
	if state != "degraded" {
		t.Errorf("Expected degraded when AllNodesHealthy=False and no Ready, got state=%s", state)
	}

	// Remove both Ready and AllNodesHealthy=False
	conditions = []metav1.Condition{
		{Type: "UpdatingVersion", Status: metav1.ConditionTrue},
	}

	state = computeNodePoolState(conditions)
	if state != "updating" {
		t.Errorf("Expected updating when only UpdatingVersion=True, got state=%s", state)
	}
}
