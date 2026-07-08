package k8s

type LoadBalancerPoolMetadata struct {
	Name        string            `json:"name"`
	Annotations map[string]string `json:"annotations"`
	Labels      map[string]string `json:"labels"`
}

type LoadBalancerPoolSpec struct {
	IPv4      string   `json:"ipv4"`
	IPv6      string   `json:"ipv6"`
	ExcludeV4 []string `json:"excludev4"`
	ExcludeV6 []string `json:"excludev6"`
}

type LoadBalancerPool struct {
	ApiVersion string                   `json:"apiVersion"`
	Kind       string                   `json:"kind"`
	Metadata   LoadBalancerPoolMetadata `json:"metadata"`
	Spec       LoadBalancerPoolSpec     `json:"spec"`
}

type LoadBalancerPoolList struct {
	ApiVersion string             `json:"apiVersion"`
	Items      []LoadBalancerPool `json:"items"`
}
