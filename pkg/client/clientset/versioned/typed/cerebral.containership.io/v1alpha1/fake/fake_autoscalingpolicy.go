/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1alpha1 "github.com/containership/cerebral/pkg/apis/cerebral.containership.io/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeAutoscalingPolicies implements AutoscalingPolicyInterface
type FakeAutoscalingPolicies struct {
	Fake *FakeCerebralV1alpha1
}

var autoscalingpoliciesResource = schema.GroupVersionResource{Group: "cerebral.containership.io", Version: "v1alpha1", Resource: "autoscalingpolicies"}

var autoscalingpoliciesKind = schema.GroupVersionKind{Group: "cerebral.containership.io", Version: "v1alpha1", Kind: "AutoscalingPolicy"}

// Get takes name of the autoscalingPolicy, and returns the corresponding autoscalingPolicy object, and an error if there is any.
func (c *FakeAutoscalingPolicies) Get(name string, options v1.GetOptions) (result *v1alpha1.AutoscalingPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(autoscalingpoliciesResource, name), &v1alpha1.AutoscalingPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AutoscalingPolicy), err
}

// List takes label and field selectors, and returns the list of AutoscalingPolicies that match those selectors.
func (c *FakeAutoscalingPolicies) List(opts v1.ListOptions) (result *v1alpha1.AutoscalingPolicyList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(autoscalingpoliciesResource, autoscalingpoliciesKind, opts), &v1alpha1.AutoscalingPolicyList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.AutoscalingPolicyList{ListMeta: obj.(*v1alpha1.AutoscalingPolicyList).ListMeta}
	for _, item := range obj.(*v1alpha1.AutoscalingPolicyList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested autoscalingPolicies.
func (c *FakeAutoscalingPolicies) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(autoscalingpoliciesResource, opts))
}

// Create takes the representation of a autoscalingPolicy and creates it.  Returns the server's representation of the autoscalingPolicy, and an error, if there is any.
func (c *FakeAutoscalingPolicies) Create(autoscalingPolicy *v1alpha1.AutoscalingPolicy) (result *v1alpha1.AutoscalingPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(autoscalingpoliciesResource, autoscalingPolicy), &v1alpha1.AutoscalingPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AutoscalingPolicy), err
}

// Update takes the representation of a autoscalingPolicy and updates it. Returns the server's representation of the autoscalingPolicy, and an error, if there is any.
func (c *FakeAutoscalingPolicies) Update(autoscalingPolicy *v1alpha1.AutoscalingPolicy) (result *v1alpha1.AutoscalingPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(autoscalingpoliciesResource, autoscalingPolicy), &v1alpha1.AutoscalingPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AutoscalingPolicy), err
}

// Delete takes name of the autoscalingPolicy and deletes it. Returns an error if one occurs.
func (c *FakeAutoscalingPolicies) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(autoscalingpoliciesResource, name), &v1alpha1.AutoscalingPolicy{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeAutoscalingPolicies) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(autoscalingpoliciesResource, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.AutoscalingPolicyList{})
	return err
}

// Patch applies the patch and returns the patched autoscalingPolicy.
func (c *FakeAutoscalingPolicies) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.AutoscalingPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(autoscalingpoliciesResource, name, pt, data, subresources...), &v1alpha1.AutoscalingPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.AutoscalingPolicy), err
}
