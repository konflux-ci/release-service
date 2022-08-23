package kcp

// NamespaceReference holds a reference to a specific namespace within a cluster using KCP.
type NamespaceReference struct {
	// Namespace references a namespace within the cluster
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +required
	Namespace string `json:"namespace"`

	// Workspace references a KCP workspace within the cluster
	// +kubebuilder:validation:Pattern="^[a-z0-9]([-a-z0-9]*[a-z0-9])(:[a-z0-9]([-a-z0-9]*[a-z0-9]))*$"
	// +optional
	Workspace string `json:"workspace,omitempty"`
}
