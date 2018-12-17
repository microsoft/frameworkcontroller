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

package v1

import (
	"os"
	"fmt"
	"io/ioutil"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"github.com/microsoft/frameworkcontroller/pkg/common"
)

type Config struct {
	// KubeApiServerAddress is default to ${KUBE_APISERVER_ADDRESS}.
	// KubeConfigFilePath is default to ${KUBECONFIG} then falls back to ${HOME}/.kube/config.
	//
	// If both KubeApiServerAddress and KubeConfigFilePath after defaulting are still empty, falls back to the
	// [k8s inClusterConfig](https://kubernetes.io/docs/tasks/access-application-cluster/access-cluster/#accessing-the-api-from-a-pod).
	//
	// If both KubeApiServerAddress and KubeConfigFilePath after defaulting are not empty,
	// KubeApiServerAddress overrides the server address specified in the file referred by KubeConfigFilePath.
	//
	// If only KubeApiServerAddress after defaulting is not empty, it should be an insecure ApiServer address (can be got from
	// [Insecure ApiServer](https://kubernetes.io/docs/reference/access-authn-authz/controlling-access/#api-server-ports-and-ips) or
	// [kubectl proxy](https://kubernetes.io/docs/tasks/access-application-cluster/access-cluster/#using-kubectl-proxy))
	// which does not enforce authentication.
	//
	// If only KubeConfigFilePath after defaulting is not empty, it should be an valid
	// [KubeConfig File](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/#explore-the-home-kube-directory)
	// which inlines or refers the valid
	// [ApiServer Credential Files](https://kubernetes.io/docs/reference/access-authn-authz/controlling-access/#transport-security).
	//
	// Address should be in format http[s]://host:port
	KubeApiServerAddress *string `yaml:"kubeApiServerAddress"`
	KubeConfigFilePath *string `yaml:"kubeConfigFilePath"`

	// Number of concurrent workers to process each different Frameworks
	WorkerNumber *int32 `yaml:"workerNumber"`

	// Check interval and timeout to expect the created CRD to be in Established condition.
	CRDEstablishedCheckIntervalSec *int64 `yaml:"crdEstablishedCheckIntervalSec"`
	CRDEstablishedCheckTimeoutSec  *int64 `yaml:"crdEstablishedCheckTimeoutSec"`

	// Timeout to expect the created object in ApiServer also appears in the local
	// cache of the Controller's Informer.
	// If the created object does not appear in the local cache within the timeout,
	// it is considered as deleted.
	ObjectLocalCacheCreationTimeoutSec *int64 `yaml:"objectLocalCacheCreationTimeoutSec"`

	// If the Framework FancyRetryPolicy is enabled and its FrameworkAttempt is
	// completed with Transient Conflict Failed CompletionType, it will be retried
	// after a random delay within this range.
	// This helps to avoid the resource deadlock for Framework which needs
	// Gang Execution, i.e. all Tasks in the Framework should be executed in an
	// all-or-nothing fashion in order to perform any useful work.
	FrameworkMinRetryDelaySecForTransientConflictFailed *int64 `yaml:"frameworkMinRetryDelaySecForTransientConflictFailed"`
	FrameworkMaxRetryDelaySecForTransientConflictFailed *int64 `yaml:"frameworkMaxRetryDelaySecForTransientConflictFailed"`
}

func NewConfig() *Config {
	c := initConfig()

	// Defaulting
	if c.KubeApiServerAddress == nil {
		c.KubeApiServerAddress = common.PtrString(EnvValueKubeApiServerAddress)
	}
	if c.KubeConfigFilePath == nil {
		c.KubeConfigFilePath = defaultKubeConfigFilePath()
	}
	if c.WorkerNumber == nil {
		c.WorkerNumber = common.PtrInt32(10)
	}
	if c.CRDEstablishedCheckIntervalSec == nil {
		c.CRDEstablishedCheckIntervalSec = common.PtrInt64(1)
	}
	if c.CRDEstablishedCheckTimeoutSec == nil {
		c.CRDEstablishedCheckTimeoutSec = common.PtrInt64(60)
	}
	if c.ObjectLocalCacheCreationTimeoutSec == nil {
		// Default to k8s.io/kubernetes/pkg/controller.ExpectationsTimeout
		c.ObjectLocalCacheCreationTimeoutSec = common.PtrInt64(5 * 60)
	}
	if c.FrameworkMinRetryDelaySecForTransientConflictFailed == nil {
		c.FrameworkMinRetryDelaySecForTransientConflictFailed = common.PtrInt64(60)
	}
	if c.FrameworkMaxRetryDelaySecForTransientConflictFailed == nil {
		c.FrameworkMaxRetryDelaySecForTransientConflictFailed = common.PtrInt64(15 * 60)
	}

	// Validation
	errPrefix := "Config Validation Failed: "
	if *c.WorkerNumber <= 0 {
		panic(fmt.Errorf(errPrefix+
				"WorkerNumber %v should be positive",
			*c.WorkerNumber))
	}
	if *c.CRDEstablishedCheckIntervalSec < 1 {
		panic(fmt.Errorf(errPrefix+
				"CRDEstablishedCheckIntervalSec %v should not be less than 1",
			*c.CRDEstablishedCheckIntervalSec))
	}
	if *c.CRDEstablishedCheckTimeoutSec < 10 {
		panic(fmt.Errorf(errPrefix+
				"CRDEstablishedCheckTimeoutSec %v should not be less than 10",
			*c.CRDEstablishedCheckTimeoutSec))
	}
	if *c.ObjectLocalCacheCreationTimeoutSec < 60 {
		panic(fmt.Errorf(errPrefix+
				"ObjectLocalCacheCreationTimeoutSec %v should not be less than 60",
			*c.ObjectLocalCacheCreationTimeoutSec))
	}
	if *c.FrameworkMinRetryDelaySecForTransientConflictFailed < 0 {
		panic(fmt.Errorf(errPrefix+
				"FrameworkMinRetryDelaySecForTransientConflictFailed %v should not be negative",
			*c.FrameworkMinRetryDelaySecForTransientConflictFailed))
	}
	if *c.FrameworkMaxRetryDelaySecForTransientConflictFailed <
			*c.FrameworkMinRetryDelaySecForTransientConflictFailed {
		panic(fmt.Errorf(errPrefix+
				"FrameworkMaxRetryDelaySecForTransientConflictFailed %v should not be less than "+
				"FrameworkMinRetryDelaySecForTransientConflictFailed %v",
			*c.FrameworkMaxRetryDelaySecForTransientConflictFailed,
			*c.FrameworkMinRetryDelaySecForTransientConflictFailed))
	}

	return c
}

func defaultKubeConfigFilePath() *string {
	configPath := EnvValueKubeConfigFilePath
	_, err := os.Stat(configPath)
	if err == nil {
		return &configPath
	}

	configPath = DefaultKubeConfigFilePath
	_, err = os.Stat(configPath)
	if err == nil {
		return &configPath
	}

	configPath = ""
	return &configPath
}

func initConfig() *Config {
	c := Config{}

	yamlBytes, err := ioutil.ReadFile(ConfigFilePath)
	if err != nil {
		panic(fmt.Errorf(
			"Failed to read config file: %v, %v", ConfigFilePath, err))
	}

	common.FromYaml(string(yamlBytes), &c)
	return &c
}

func BuildKubeConfig(cConfig *Config) (*rest.Config) {
	kConfig, err := clientcmd.BuildConfigFromFlags(
		*cConfig.KubeApiServerAddress, *cConfig.KubeConfigFilePath)
	if err != nil {
		panic(fmt.Errorf("Failed to build KubeConfig, please ensure "+
				"config kubeApiServerAddress or config kubeConfigFilePath or "+
				"${KUBE_APISERVER_ADDRESS} or ${KUBECONFIG} or ${HOME}/.kube/config or "+
				"${KUBERNETES_SERVICE_HOST}:${KUBERNETES_SERVICE_PORT} is valid: "+
				"Error: %v", err))
	}
	return kConfig
}
