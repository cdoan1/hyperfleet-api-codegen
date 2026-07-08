package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodePool represents a HyperFleet managed NodePool for a cluster
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
type NodePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodePoolSpec   `json:"spec"`
	Status NodePoolStatus `json:"status,omitempty"`
}

// NodePoolSpec defines the desired state of a NodePool
type NodePoolSpec struct {
	// === HyperFleet Envelope Fields ===

	// ClusterRef references the parent Cluster
	// +kubebuilder:validation:Required
	ClusterRef ClusterReference `json:"clusterRef"`

	// DisplayName is a human-readable name for the node pool
	// +hyperfleet:write-mode=mutable
	DisplayName string `json:"displayName,omitempty"`

	// AutoRepair enables automatic repair of unhealthy nodes
	// +hyperfleet:write-mode=mutable
	AutoRepair *bool `json:"autoRepair,omitempty"`

	// Labels to apply to nodes in this pool
	// +hyperfleet:write-mode=mutable
	Labels map[string]string `json:"labels,omitempty"`

	// AccountID identifies the customer account (platform-managed, hidden)
	// +k8s:openapi-gen=false
	// +hyperfleet:write-mode=service-set
	AccountID string `json:"accountId,omitempty"`

	// InternalPoolID is an internal platform identifier (platform-managed, hidden)
	// +k8s:openapi-gen=false
	// +hyperfleet:write-mode=service-set
	InternalPoolID string `json:"internalPoolId,omitempty"`

	// === HyperShift Passthrough ===

	// NodePool contains the full HyperShift NodePool configuration
	// All fields are generated from upstream and have safe defaults (hidden + service-set)
	// +kubebuilder:validation:Required
	NodePool NodePoolSpecPassthrough `json:"nodePool"`
}

// ClusterReference identifies the parent cluster
type ClusterReference struct {
	// Name is the name of the Cluster resource
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace is the namespace of the Cluster resource
	// If empty, defaults to the same namespace as this NodePool
	Namespace string `json:"namespace,omitempty"`
}

// NodePoolStatus defines the observed state of a NodePool
type NodePoolStatus struct {
	// State represents the high-level node pool state
	// +kubebuilder:validation:Enum=pending;scaling;ready;degraded;deleting;failed
	State string `json:"state,omitempty"`

	// Conditions represent detailed node pool status
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Replicas is the current number of nodes
	Replicas int32 `json:"replicas,omitempty"`

	// ReadyReplicas is the number of ready nodes
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// AvailableReplicas is the number of available nodes
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`
}

// NodePoolList contains a list of NodePools
// +kubebuilder:object:root=true
type NodePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodePool `json:"items"`
}
