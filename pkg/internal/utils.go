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

package internal

import (
	"fmt"
	kubebatchclient "github.com/kubernetes-sigs/kube-batch/pkg/client/clientset/versioned"
	ci "github.com/microsoft/frameworkcontroller/pkg/apis/frameworkcontroller/v1"
	frameworkClient "github.com/microsoft/frameworkcontroller/pkg/client/clientset/versioned"
	"github.com/microsoft/frameworkcontroller/pkg/common"
	core "k8s.io/api/core/v1"
	apiExtensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiClient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeClient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	"reflect"
)

func CreateClients(kConfig *rest.Config) (
	kubeClient.Interface, frameworkClient.Interface,kubebatchclient.Interface) {
	kClient, err := kubeClient.NewForConfig(kConfig)
	if err != nil {
		panic(fmt.Errorf("Failed to create KubeClient: %v", err))
	}

	fClient, err := frameworkClient.NewForConfig(kConfig)
	if err != nil {
		panic(fmt.Errorf("Failed to create FrameworkClient: %v", err))
	}
	kbClient, err := kubebatchclient.NewForConfig(kConfig)
	if err != nil {
		panic(fmt.Errorf("Failed to create KubeBatchClient: %v", err))
	}
	return kClient, fClient,kbClient
}

func PutCRD(
	config *rest.Config, crd *apiExtensions.CustomResourceDefinition,
	establishedCheckIntervalSec *int64, establishedCheckTimeoutSec *int64) {
	client := createCRDClient(config)

	err := putCRDInternal(client, crd, establishedCheckIntervalSec, establishedCheckTimeoutSec)
	if err != nil {
		panic(fmt.Errorf("Failed to put CRD: %v", err))
	} else {
		klog.Infof("Succeeded to put CRD")
	}
}

func DeleteCRD(config *rest.Config, name string) {
	client := createCRDClient(config)

	err := client.ApiextensionsV1beta1().CustomResourceDefinitions().Delete(name, meta.NewDeleteOptions(0))
	if err != nil && !apiErrors.IsNotFound(err) {
		panic(fmt.Errorf("Failed to delete CRD: %v", err))
	} else {
		klog.Infof("Succeeded to delete CRD")
	}
}

func createCRDClient(config *rest.Config) apiClient.Interface {
	client, err := apiClient.NewForConfig(config)
	if err != nil {
		panic(fmt.Errorf("Failed to create CRDClient: %v", err))
	}

	return client
}

func putCRDInternal(
	client apiClient.Interface, newCRD *apiExtensions.CustomResourceDefinition,
	establishedCheckIntervalSec *int64, establishedCheckTimeoutSec *int64) error {

	remoteCRD, err := client.ApiextensionsV1beta1().CustomResourceDefinitions().Get(newCRD.Name, meta.GetOptions{})
	if err == nil {
		klog.Infof("Update CRD %v", newCRD.Name)
		if !reflect.DeepEqual(remoteCRD.Spec, newCRD.Spec) {
			updateCRD := remoteCRD
			updateCRD.Spec = newCRD.Spec
			remoteCRD, err = client.ApiextensionsV1beta1().CustomResourceDefinitions().Update(updateCRD)
			if err != nil {
				return err
			}
		}
	} else if apiErrors.IsNotFound(err) {
		klog.Infof("Create CRD %v", newCRD.Name)
		remoteCRD, err = client.ApiextensionsV1beta1().CustomResourceDefinitions().Create(newCRD)
		if err != nil {
			return err
		}
	} else {
		return err
	}

	if isCRDEstablished(remoteCRD) {
		return nil
	}
	return wait.Poll(
		common.SecToDuration(establishedCheckIntervalSec),
		common.SecToDuration(establishedCheckTimeoutSec),
		func() (bool, error) {
			remoteCRD, err = client.ApiextensionsV1beta1().CustomResourceDefinitions().Get(newCRD.Name, meta.GetOptions{})
			if err != nil {
				return false, err
			}
			return isCRDEstablished(remoteCRD), nil
		})
}

func isCRDEstablished(crd *apiExtensions.CustomResourceDefinition) bool {
	for _, cond := range crd.Status.Conditions {
		if cond.Status == apiExtensions.ConditionTrue &&
			cond.Type == apiExtensions.Established {
			return true
		}
	}
	return false
}

func GetKey(obj interface{}) (string, error) {
	return cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
}

func SplitKey(key string) (namespace, name string, err error) {
	return cache.SplitMetaNamespaceKey(key)
}

// obj could be *core.Framework or cache.DeletedFinalStateUnknown.
func ToFramework(obj interface{}) *ci.Framework {
	f, ok := obj.(*ci.Framework)

	if !ok {
		deletedFinalStateUnknown, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.Errorf(
				"Failed to convert obj to Framework or DeletedFinalStateUnknown: %#v",
				obj)
			return nil
		}

		f, ok = deletedFinalStateUnknown.Obj.(*ci.Framework)
		if !ok {
			klog.Errorf(
				"Failed to convert DeletedFinalStateUnknown.Obj to Framework: %#v",
				deletedFinalStateUnknown)
			return nil
		}
	}

	return f
}

// obj could be *core.ConfigMap or cache.DeletedFinalStateUnknown.
func ToConfigMap(obj interface{}) *core.ConfigMap {
	cm, ok := obj.(*core.ConfigMap)

	if !ok {
		deletedFinalStateUnknown, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.Errorf(
				"Failed to convert obj to ConfigMap or DeletedFinalStateUnknown: %#v",
				obj)
			return nil
		}

		cm, ok = deletedFinalStateUnknown.Obj.(*core.ConfigMap)
		if !ok {
			klog.Errorf(
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
			klog.Errorf(
				"Failed to convert obj to Pod or DeletedFinalStateUnknown: %#v",
				obj)
			return nil
		}

		pod, ok = deletedFinalStateUnknown.Obj.(*core.Pod)
		if !ok {
			klog.Errorf(
				"Failed to convert DeletedFinalStateUnknown.Obj to Pod: %#v",
				deletedFinalStateUnknown)
			return nil
		}
	}

	return pod
}
