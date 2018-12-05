package nodeutil

import (
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

// GetNodesLabelSelector creates a selector object from the passed in labels map
func GetNodesLabelSelector(labelsMap map[string]string) labels.Selector {
	selector := labels.NewSelector()
	for key, value := range labelsMap {
		l, _ := labels.NewRequirement(key, selection.Equals, []string{value})
		selector = selector.Add(*l)
	}

	return selector
}
