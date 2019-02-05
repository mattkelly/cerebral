package kubernetes

// Metric is a metric exposed by this backend
type Metric int

const (
	// MetricCPUPercentAllocation is used to gather info about the CPU allocation of nodes
	MetricCPUPercentAllocation Metric = iota
	// MetricGPUPercentAllocation is used to gather info about the GPU allocation of nodes
	MetricGPUPercentAllocation
	// MetricMemoryPercentAllocation is used to gather info about the memory allocation of nodes
	MetricMemoryPercentAllocation
	// MetricEphemeralStoragePercentAllocation is used to gather info about the disk allocation of nodes
	MetricEphemeralStoragePercentAllocation
	// MetricPodPercentAllocation is used to gather info about the Pod allocation of nodes
	MetricPodPercentAllocation
)

// GPUVendors returns array of supported GPU vendors
func GPUVendors() [2]string {
	return [...]string{
		"amd.com/gpu",
		"nvidia.com/gpu",
	}
}

// String is a stringer for Metric
func (m Metric) String() string {
	switch m {
	case MetricCPUPercentAllocation:
		return "cpu_percent_allocation"
	case MetricGPUPercentAllocation:
		return "gpu_percent_allocation"
	case MetricMemoryPercentAllocation:
		return "memory_percent_allocation"
	case MetricEphemeralStoragePercentAllocation:
		return "ephemeral_storage_percent_allocation"
	case MetricPodPercentAllocation:
		return "pod_percent_allocation"
	}

	return "unknown"
}
