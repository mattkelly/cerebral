package metrics

import (
	corev1 "k8s.io/api/core/v1"
)

// A Backend is used to interface with a metrics backend.
type Backend interface {
	// GetValue queries the backend and returns the raw numerical value of the
	// requested metric (with the given configuration) for the given nodes
	// at this point in time.
	GetValue(metric string, configuration map[string]string, nodes []corev1.Node) (float64, error)
}
