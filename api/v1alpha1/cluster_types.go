package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Cluster represents a HyperFleet managed OpenShift cluster
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec"`
	Status ClusterStatus `json:"status,omitempty"`
}

// ClusterSpec defines the desired state of a Cluster
type ClusterSpec struct {
	// === HyperFleet Envelope Fields ===
	// These are HyperFleet-specific fields that wrap the HyperShift cluster

	// DisplayName is a human-readable name for the cluster
	// +hyperfleet:write-mode=mutable
	// +kubebuilder:validation:MaxLength=256
	DisplayName string `json:"displayName,omitempty"`

	// DeleteProtection prevents accidental deletion when enabled
	// +hyperfleet:write-mode=mutable
	DeleteProtection *bool `json:"deleteProtection,omitempty"`

	// ExpirationTimestamp marks when this cluster should be automatically deleted
	// +hyperfleet:write-mode=mutable
	ExpirationTimestamp *metav1.Time `json:"expirationTimestamp,omitempty"`

	// Properties are arbitrary key-value pairs for customer metadata
	// +hyperfleet:write-mode=mutable
	Properties map[string]string `json:"properties,omitempty"`

	// Tags are customer-defined labels for organizational purposes
	// This is a TechPreview feature
	// +hyperfleet:write-mode=mutable
	// +openshift:enable:FeatureGate=HyperFleetAutoScaling
	Tags map[string]string `json:"tags,omitempty"`

	// AccountID identifies the customer account (platform-managed, hidden from API)
	// +k8s:openapi-gen=false
	// +hyperfleet:write-mode=service-set
	AccountID string `json:"accountId,omitempty"`

	// CreatorARN is the AWS ARN of the user who created this cluster (platform-managed, hidden)
	// +k8s:openapi-gen=false
	// +hyperfleet:write-mode=service-set
	CreatorARN string `json:"creatorARN,omitempty"`

	// InternalID is an internal platform identifier (platform-managed, hidden)
	// +k8s:openapi-gen=false
	// +hyperfleet:write-mode=service-set
	InternalID string `json:"internalId,omitempty"`

	// === HyperShift Passthrough ===
	// This embeds all upstream HyperShift HostedCluster fields

	// HostedCluster contains the full HyperShift HostedCluster configuration
	// All fields are generated from upstream and have safe defaults (hidden + service-set)
	// until explicitly reviewed and exposed
	// +kubebuilder:validation:Required
	HostedCluster HostedClusterSpecPassthrough `json:"hostedCluster"`
}

// ClusterStatus defines the observed state of a Cluster
type ClusterStatus struct {
	// State represents the high-level cluster state
	// +kubebuilder:validation:Enum=pending;provisioning;ready;degraded;deleting;failed
	State string `json:"state,omitempty"`

	// Conditions represent detailed cluster status
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Version is the observed OpenShift version
	Version string `json:"version,omitempty"`

	// APIEndpoint is the cluster API server endpoint
	APIEndpoint string `json:"apiEndpoint,omitempty"`

	// ConsoleURL is the web console URL
	ConsoleURL string `json:"consoleUrl,omitempty"`

	// ProvisionStartTime is when provisioning began
	ProvisionStartTime *metav1.Time `json:"provisionStartTime,omitempty"`

	// ReadyTime is when the cluster became ready
	ReadyTime *metav1.Time `json:"readyTime,omitempty"`
}

// ClusterList contains a list of Clusters
// +kubebuilder:object:root=true
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}
