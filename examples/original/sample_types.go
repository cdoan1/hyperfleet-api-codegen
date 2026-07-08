package examples

// ClusterSpec defines the desired state of a Cluster
type ClusterSpec struct {
	// Customer can set and change
	// +hyperfleet:write-mode=mutable
	DeleteProtection *bool `json:"deleteProtection,omitempty"`

	// Customer sets on create, cannot change
	// +hyperfleet:write-mode=immutable
	Name string `json:"name"`

	// Platform sets based on regional endpoint
	// +hyperfleet:write-mode=service-set
	Region string `json:"region"`

	// Platform sets, customer cannot see
	// +k8s:openapi-gen=false
	// +hyperfleet:write-mode=service-set
	AccountID string `json:"accountId"`

	// Platform sets, customer cannot see
	// +k8s:openapi-gen=false
	// +hyperfleet:write-mode=service-set
	CreatorARN string `json:"creatorARN,omitempty"`

	// Passthrough to HyperShift HostedCluster
	// +kubebuilder:validation:Required
	HostedCluster HostedClusterPassthrough `json:"hostedCluster"`
}

// HostedClusterPassthrough mirrors HyperShift HostedCluster fields
type HostedClusterPassthrough struct {
	// Available to all customers (GA, no gate)
	// +hyperfleet:write-mode=immutable
	// +kubebuilder:validation:Required
	Release Release `json:"release"`

	// Only available to customers with this gate enabled
	// +openshift:enable:FeatureGate=HyperFleetEtcdConfig
	// +hyperfleet:write-mode=immutable
	// +optional
	Etcd *EtcdSpec `json:"etcd,omitempty"`
}

// Release specifies the OpenShift release
type Release struct {
	// +hyperfleet:write-mode=immutable
	Image string `json:"image"`
}

// EtcdSpec configures etcd management
type EtcdSpec struct {
	// +hyperfleet:write-mode=immutable
	ManagementType string `json:"managementType"`
}
