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
	"fmt"
	"github.com/microsoft/frameworkcontroller/pkg/common"
	"io/ioutil"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
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
	KubeConfigFilePath   *string `yaml:"kubeConfigFilePath"`

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

	// A Framework will only be retained within recent FrameworkCompletedRetainSec
	// after it is completed, i.e. it will be automatically deleted after
	// f.Status.CompletionTime + FrameworkCompletedRetainSec.
	FrameworkCompletedRetainSec *int64 `yaml:"frameworkCompletedRetainSec"`

	// If the Framework FancyRetryPolicy is enabled and its FrameworkAttempt is
	// completed with Transient Conflict Failed CompletionType, it will be retried
	// after a random delay within this range.
	// This helps to avoid the resource deadlock for Framework which needs
	// Gang Execution, i.e. all Tasks in the Framework should be executed in an
	// all-or-nothing fashion in order to perform any useful work.
	FrameworkMinRetryDelaySecForTransientConflictFailed *int64 `yaml:"frameworkMinRetryDelaySecForTransientConflictFailed"`
	FrameworkMaxRetryDelaySecForTransientConflictFailed *int64 `yaml:"frameworkMaxRetryDelaySecForTransientConflictFailed"`

	// Specify when to log the snapshot of which managed object.
	// This enables external systems to collect and process the history snapshots,
	// such as persistence, metrics conversion, visualization, alerting, acting,
	// analysis, etc.
	// Notes:
	// 1. The snapshot is logged to stderr and can be extracted by the regular
	//    expression ": ObjectSnapshot: (.+)".
	// 2. To determine the type of the snapshot, using object.apiVersion and
	//    object.kind.
	// 3. The same snapshot may be logged more than once in some rare cases, so
	//    external systems may need to deduplicate them by object.resourceVersion.
	// 4. The snapshot triggered by deletion may be missed to log during the
	//    FrameworkController downtime.
	LogObjectSnapshot LogObjectSnapshot `yaml:"logObjectSnapshot"`

	// Specify how to classify and summarize Pod failures:
	// 1. Generate universally unique and comparable CompletionCode.
	// 2. Generate CompletionType to instruct FancyRetryPolicy.
	// 3. Generate Diagnostics to summarize the Pod failure.
	// Notes:
	// 1. It will only be used for Failed Pods.
	// 2. Before it is used, it will be appended to the predefined internal
	//    CompletionCodeInfoList and if multiple CompletionCodes are matched in
	//    the final CompletionCodeInfoList, prefer to pick the first one.
	// 3. Non-positive CompletionCode must be universally unique and comparable
	//    since it can only be generated from FrameworkController, but positive
	//    CompletionCode may not since it may also be generated from Container
	//    ExitCode. So, it still needs the cooperation from Container to ensure
	//    positive CompletionCode is also universally unique and comparable.
	PodFailureSpec []*CompletionCodeInfo `yaml:"podFailureSpec"`
	// add bool EnableKubeBatch
	EnableKubeBatch *bool `yaml:"enableKubeBatch"`
}

type LogObjectSnapshot struct {
	Framework LogFrameworkSnapshot `yaml:"framework"`
	Pod       LogPodSnapshot       `yaml:"pod"`
}

type LogFrameworkSnapshot struct {
	OnTaskRetry         *bool `yaml:"onTaskRetry"`
	OnFrameworkRetry    *bool `yaml:"onFrameworkRetry"`
	OnFrameworkDeletion *bool `yaml:"onFrameworkDeletion"`
}

type LogPodSnapshot struct {
	OnPodDeletion *bool `yaml:"onPodDeletion"`
}

type CompletionCodeInfo struct {
	// Must not duplicate with other codes.
	// It should not be within [-999, 0] and [200, 219], if it is from Config.
	// It can universally locate the CompletionCodeInfo, if it is non-positive.
	Code *CompletionCode `yaml:"code"`
	// The textual phrase representation of the CompletionCode.
	// Default to empty.
	Phrase CompletionPhrase `yaml:"phrase,omitempty"`
	// The CompletionTypeName must be Failed, if it is from Config.
	// Default to Failed.
	Type CompletionType `yaml:"type"`
	// If the Pod matches ANY pattern in the PodPatterns, it matches the
	// CompletionCode.
	// It is required, if it is from Config.
	// Default to match NONE.
	PodPatterns []*PodPattern `yaml:"podPatterns,omitempty"`
}

// Used to match against the corresponding fields in Pod object.
// ALL fields are optional and default to match ANY, except for explict comments.
// The pattern is matched if and only if ALL fields are matched.
type PodPattern struct {
	NameRegex    string              `yaml:"nameRegex,omitempty"`
	ReasonRegex  string              `yaml:"reasonRegex,omitempty"`
	MessageRegex string              `yaml:"messageRegex,omitempty"`
	Containers   []*ContainerPattern `yaml:"containers,omitempty"`
}

type ContainerPattern struct {
	NameRegex    string     `yaml:"nameRegex,omitempty"`
	ReasonRegex  string     `yaml:"reasonRegex,omitempty"`
	MessageRegex string     `yaml:"messageRegex,omitempty"`
	SignalRange  Int32Range `yaml:"signalRange,omitempty"`
	// It is the range of Container ExitCode.
	// Default to [1, nil], if it is from Config.
	CodeRange Int32Range `yaml:"codeRange,omitempty"`
}

// Represent [Min, Max] and nil indicates unlimited.
type Int32Range struct {
	Min *int32 `yaml:"min,omitempty"`
	Max *int32 `yaml:"max,omitempty"`
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
	if c.FrameworkCompletedRetainSec == nil {
		c.FrameworkCompletedRetainSec = common.PtrInt64(30 * 24 * 3600)
	}
	if c.FrameworkMinRetryDelaySecForTransientConflictFailed == nil {
		c.FrameworkMinRetryDelaySecForTransientConflictFailed = common.PtrInt64(60)
	}
	if c.FrameworkMaxRetryDelaySecForTransientConflictFailed == nil {
		c.FrameworkMaxRetryDelaySecForTransientConflictFailed = common.PtrInt64(15 * 60)
	}
	if c.LogObjectSnapshot.Framework.OnTaskRetry == nil {
		c.LogObjectSnapshot.Framework.OnTaskRetry = common.PtrBool(true)
	}
	if c.LogObjectSnapshot.Framework.OnFrameworkRetry == nil {
		c.LogObjectSnapshot.Framework.OnFrameworkRetry = common.PtrBool(true)
	}
	if c.LogObjectSnapshot.Framework.OnFrameworkDeletion == nil {
		c.LogObjectSnapshot.Framework.OnFrameworkDeletion = common.PtrBool(true)
	}
	if c.LogObjectSnapshot.Pod.OnPodDeletion == nil {
		c.LogObjectSnapshot.Pod.OnPodDeletion = common.PtrBool(true)
	}
	if c.EnableKubeBatch == nil {
		c.EnableKubeBatch = common.PtrBool(true)
	}
	for _, codeInfo := range c.PodFailureSpec {
		if codeInfo.Type.Name == "" {
			codeInfo.Type.Name = CompletionTypeNameFailed
		}
		for _, podPattern := range codeInfo.PodPatterns {
			for _, containerPattern := range podPattern.Containers {
				if containerPattern.CodeRange.Min == nil {
					containerPattern.CodeRange.Min = common.PtrInt32(1)
				}
			}
		}
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
	codeInfoMap := map[CompletionCode]*CompletionCodeInfo{}
	for _, codeInfo := range c.PodFailureSpec {
		if codeInfo.Type.Name != CompletionTypeNameFailed {
			panic(fmt.Errorf(errPrefix+
				"PodFailureSpec contains CompletionTypeName which is not %v:\n%v",
				CompletionTypeNameFailed, common.ToYaml(codeInfo)))
		}
		if len(codeInfo.PodPatterns) == 0 {
			panic(fmt.Errorf(errPrefix+
				"PodFailureSpec contains empty PodPatterns:\n%v",
				common.ToYaml(codeInfo)))
		}
		if codeInfo.Code == nil {
			panic(fmt.Errorf(errPrefix+
				"PodFailureSpec contains nil CompletionCode:\n%v",
				common.ToYaml(codeInfo)))
		}
		if CompletionCodeReservedNonPositive.Contains(*codeInfo.Code) ||
			CompletionCodeReservedPositive.Contains(*codeInfo.Code) {
			panic(fmt.Errorf(errPrefix+
				"PodFailureSpec contains CompletionCode which should not be within "+
				"%v and %v:\n%v",
				CompletionCodeReservedNonPositive, CompletionCodeReservedPositive,
				common.ToYaml(codeInfo)))
		}
		if existingCodeInfo, ok := codeInfoMap[*codeInfo.Code]; ok {
			panic(fmt.Errorf(errPrefix+
				"PodFailureSpec contains duplicated CompletionCode:"+
				"\nExisting CompletionCodeInfo:\n%v,\nChecking CompletionCodeInfo:\n%v",
				common.ToYaml(existingCodeInfo), common.ToYaml(codeInfo)))
		}
		codeInfoMap[*codeInfo.Code] = codeInfo
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

func BuildKubeConfig(cConfig *Config) *rest.Config {
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
