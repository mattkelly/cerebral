/*
Copyright 2018 The Kubernetes Authors.

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

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/containership/cerebral/pkg/apis/cerebral.containership.io/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// AutoscalingGroupLister helps list AutoscalingGroups.
type AutoscalingGroupLister interface {
	// List lists all AutoscalingGroups in the indexer.
	List(selector labels.Selector) (ret []*v1alpha1.AutoscalingGroup, err error)
	// Get retrieves the AutoscalingGroup from the index for a given name.
	Get(name string) (*v1alpha1.AutoscalingGroup, error)
	AutoscalingGroupListerExpansion
}

// autoscalingGroupLister implements the AutoscalingGroupLister interface.
type autoscalingGroupLister struct {
	indexer cache.Indexer
}

// NewAutoscalingGroupLister returns a new AutoscalingGroupLister.
func NewAutoscalingGroupLister(indexer cache.Indexer) AutoscalingGroupLister {
	return &autoscalingGroupLister{indexer: indexer}
}

// List lists all AutoscalingGroups in the indexer.
func (s *autoscalingGroupLister) List(selector labels.Selector) (ret []*v1alpha1.AutoscalingGroup, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.AutoscalingGroup))
	})
	return ret, err
}

// Get retrieves the AutoscalingGroup from the index for a given name.
func (s *autoscalingGroupLister) Get(name string) (*v1alpha1.AutoscalingGroup, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("autoscalinggroup"), name)
	}
	return obj.(*v1alpha1.AutoscalingGroup), nil
}
