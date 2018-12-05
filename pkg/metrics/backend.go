package metrics

// A Backend is used to interface with a metrics backend.
type Backend interface {
	// GetValue queries the backend and returns the raw numerical value of the
	// requested metric (with the given configuration) for the given nodes
	// at this point in time.
	GetValue(metric string, configuration map[string]string, nodeSelector map[string]string) (float64, error)
}
