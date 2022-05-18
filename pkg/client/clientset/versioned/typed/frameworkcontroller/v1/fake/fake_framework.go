// MIT License
//
// Copyright (c) Microsoft Corporation. All rights reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	frameworkcontrollerv1 "github.com/microsoft/frameworkcontroller/pkg/apis/frameworkcontroller/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeFrameworks implements FrameworkInterface
type FakeFrameworks struct {
	Fake *FakeFrameworkcontrollerV1
	ns   string
}

var frameworksResource = schema.GroupVersionResource{Group: "frameworkcontroller.microsoft.com", Version: "v1", Resource: "frameworks"}

var frameworksKind = schema.GroupVersionKind{Group: "frameworkcontroller.microsoft.com", Version: "v1", Kind: "Framework"}

// Get takes name of the framework, and returns the corresponding framework object, and an error if there is any.
func (c *FakeFrameworks) Get(ctx context.Context, name string, options v1.GetOptions) (result *frameworkcontrollerv1.Framework, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(frameworksResource, c.ns, name), &frameworkcontrollerv1.Framework{})

	if obj == nil {
		return nil, err
	}
	return obj.(*frameworkcontrollerv1.Framework), err
}

// List takes label and field selectors, and returns the list of Frameworks that match those selectors.
func (c *FakeFrameworks) List(ctx context.Context, opts v1.ListOptions) (result *frameworkcontrollerv1.FrameworkList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(frameworksResource, frameworksKind, c.ns, opts), &frameworkcontrollerv1.FrameworkList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &frameworkcontrollerv1.FrameworkList{ListMeta: obj.(*frameworkcontrollerv1.FrameworkList).ListMeta}
	for _, item := range obj.(*frameworkcontrollerv1.FrameworkList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested frameworks.
func (c *FakeFrameworks) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(frameworksResource, c.ns, opts))

}

// Create takes the representation of a framework and creates it.  Returns the server's representation of the framework, and an error, if there is any.
func (c *FakeFrameworks) Create(ctx context.Context, framework *frameworkcontrollerv1.Framework, opts v1.CreateOptions) (result *frameworkcontrollerv1.Framework, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(frameworksResource, c.ns, framework), &frameworkcontrollerv1.Framework{})

	if obj == nil {
		return nil, err
	}
	return obj.(*frameworkcontrollerv1.Framework), err
}

// Update takes the representation of a framework and updates it. Returns the server's representation of the framework, and an error, if there is any.
func (c *FakeFrameworks) Update(ctx context.Context, framework *frameworkcontrollerv1.Framework, opts v1.UpdateOptions) (result *frameworkcontrollerv1.Framework, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(frameworksResource, c.ns, framework), &frameworkcontrollerv1.Framework{})

	if obj == nil {
		return nil, err
	}
	return obj.(*frameworkcontrollerv1.Framework), err
}

// Delete takes name of the framework and deletes it. Returns an error if one occurs.
func (c *FakeFrameworks) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(frameworksResource, c.ns, name), &frameworkcontrollerv1.Framework{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeFrameworks) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(frameworksResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &frameworkcontrollerv1.FrameworkList{})
	return err
}

// Patch applies the patch and returns the patched framework.
func (c *FakeFrameworks) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *frameworkcontrollerv1.Framework, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(frameworksResource, c.ns, name, pt, data, subresources...), &frameworkcontrollerv1.Framework{})

	if obj == nil {
		return nil, err
	}
	return obj.(*frameworkcontrollerv1.Framework), err
}
