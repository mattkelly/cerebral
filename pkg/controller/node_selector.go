package controller

import (
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	cerebralv1alpha1 "github.com/containership/cerebral/pkg/apis/cerebral.containership.io/v1alpha1"
)

// getNodesLabelSelector creates a selector object from the passed in labels map
func getNodesLabelSelector(labelsMap map[string]string) labels.Selector {
	selector := labels.NewSelector()
	for key, value := range labelsMap {
		l, _ := labels.NewRequirement(key, selection.Equals, []string{value})
		selector = selector.Add(*l)
	}

	return selector
}

// findNodesAGs goes through each autoscaling group and checks to see if the AG
// nodeSelector matches the node labels passed into the function returning all
// AGs that match
func findAGsMatchingNodeLabels(nodeLabels map[string]string, ags []*cerebralv1alpha1.AutoscalingGroup) []*cerebralv1alpha1.AutoscalingGroup {
	matchingags := make([]*cerebralv1alpha1.AutoscalingGroup, 0)

	for _, autoscalingGroup := range ags {
		// create selector object from nodeSelector of AG
		agselectors := getNodesLabelSelector(autoscalingGroup.Spec.NodeSelector)

		// check to see if the nodeSelector labels match the node labels that
		// were passed in
		if agselectors.Matches(labels.Set(nodeLabels)) {
			matchingags = append(matchingags, autoscalingGroup)
		}
	}

	return matchingags
}
