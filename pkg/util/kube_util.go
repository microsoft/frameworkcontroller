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

package util

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/rest"
	core "k8s.io/api/core/v1"
	kubeClient "k8s.io/client-go/kubernetes"
	frameworkClient "github.com/microsoft/frameworkcontroller/pkg/client/clientset/versioned"
)

func CreateClients(kConfig *rest.Config) (
		kubeClient.Interface, frameworkClient.Interface) {
	kClient, err := kubeClient.NewForConfig(kConfig)
	if err != nil {
		panic(fmt.Errorf("Failed to create KubeClient: %v", err))
	}

	fClient, err := frameworkClient.NewForConfig(kConfig)
	if err != nil {
		panic(fmt.Errorf("Failed to create FrameworkClient: %v", err))
	}

	return kClient, fClient
}

func GetKey(obj interface{}) (string, error) {
	return cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
}

func SplitKey(key string) (namespace, name string, err error) {
	return cache.SplitMetaNamespaceKey(key)
}

// obj could be *core.ConfigMap or cache.DeletedFinalStateUnknown.
func ToConfigMap(obj interface{}) *core.ConfigMap {
	cm, ok := obj.(*core.ConfigMap)

	if !ok {
		deletedFinalStateUnknown, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			log.Errorf(
				"Failed to convert obj to ConfigMap or DeletedFinalStateUnknown: %#v",
				obj)
			return nil
		}

		cm, ok = deletedFinalStateUnknown.Obj.(*core.ConfigMap)
		if !ok {
			log.Errorf(
				"Failed to convert DeletedFinalStateUnknown.Obj to ConfigMap: %#v",
				deletedFinalStateUnknown)
			return nil
		}
	}

	return cm
}

// obj could be *core.Pod or cache.DeletedFinalStateUnknown.
func ToPod(obj interface{}) *core.Pod {
	pod, ok := obj.(*core.Pod)

	if !ok {
		deletedFinalStateUnknown, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			log.Errorf(
				"Failed to convert obj to Pod or DeletedFinalStateUnknown: %#v",
				obj)
			return nil
		}

		pod, ok = deletedFinalStateUnknown.Obj.(*core.Pod)
		if !ok {
			log.Errorf(
				"Failed to convert DeletedFinalStateUnknown.Obj to Pod: %#v",
				deletedFinalStateUnknown)
			return nil
		}
	}

	return pod
}
