package atc

// Info carries metadata about a given instance of ATC.
type Info struct {
	// The version of ATC running on this instance
	Version string `json:"version"`
	// The matching worker version of this ATC version
	WorkerVersion string `json:"worker_version"`
	// The configured external cluster URL
	ExternalURL string `json:"external_url,omitempty"`
	// The configured name of the Concourse cluster
	ClusterName string `json:"cluster_name,omitempty"`
}
