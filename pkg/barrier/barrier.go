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

package barrier

import (
	"fmt"
	ci "github.com/microsoft/frameworkcontroller/pkg/apis/frameworkcontroller/v1"
	frameworkClient "github.com/microsoft/frameworkcontroller/pkg/client/clientset/versioned"
	"github.com/microsoft/frameworkcontroller/pkg/common"
	"github.com/microsoft/frameworkcontroller/pkg/internal"
	"io/ioutil"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeClient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"os"
	"strconv"
	"strings"
)

// FrameworkController Extension: FrameworkBarrier
//
// Best Practice:
// It is usually used as the initContainer to provide a simple way to:
// 1. Do Gang Execution without resource deadlock.
//    So that the AppContainers of all Tasks in the Framework will be executed in
//    an all-or-nothing fashion without the need for Gang Scheduling.
// 2. Start the AppContainers in the Pod only after its PodUID is persisted in
//    the Framework object by FrameworkController.
//    So that the completion or deletion event of a Pod with started AppContainers
//    will never be missed by FrameworkController to further trigger RetryPolicy
//    or FrameworkAttemptCompletionPolicy.
// 3. Inject peer-to-peer service discovery information into the AppContainers.
//    So that any Task in the Framework is able to discover all other Tasks in
//    the same Framework without the need for k8s DNS.
//
// Usage:
// It waits until all Tasks in the specified Framework object are ready with not
// nil PodIP and then dumps the Framework object to local file: ./framework.json,
// besides it also generates the injector script to local file: ./injector.sh
// which provides a default way to inject some Framework information into caller
// process.
//
// ./injector.sh exports below environment variables:
// For each {TaskRoleName} in the Framework:
//   FB_{UpperCase({TaskRoleName})}_IPS=
//     {Task[0].PodIP},...,
//     {Task[TaskRole.TaskNumber-1].PodIP}
//   FB_{UpperCase({TaskRoleName})}_ADDRESSES=
//     {Task[0].PodIP}:${FB_{UpperCase({TaskRoleName})}_PORT},...,
//     {Task[TaskRole.TaskNumber-1].PodIP}:${FB_{UpperCase({TaskRoleName})}_PORT}
//   Note, the environment variable FB_{UpperCase({TaskRoleName})}_PORT should be
//   provided by the caller in advance.
//
// Caller can also write its own injector script to inject other Framework
// information from the ./framework.json.
type FrameworkBarrier struct {
	kConfig *rest.Config
	bConfig *Config

	kClient kubeClient.Interface
	fClient frameworkClient.Interface
}

///////////////////////////////////////////////////////////////////////////////////////
// Constants
///////////////////////////////////////////////////////////////////////////////////////
const (
	ComponentName           = "frameworkbarrier"
	FrameworkObjectFilePath = "./framework.json"
	InjectorFilePath        = "./injector.sh"

	EnvNameBarrierCheckIntervalSec = "BARRIER_CHECK_INTERVAL_SEC"
	EnvNameBarrierCheckTimeoutSec  = "BARRIER_CHECK_TIMEOUT_SEC"
)

///////////////////////////////////////////////////////////////////////////////////////
// Config
///////////////////////////////////////////////////////////////////////////////////////
type Config struct {
	// See the same fields in pkg/apis/frameworkcontroller/v1/config.go
	KubeApiServerAddress string `yaml:"kubeApiServerAddress"`
	KubeConfigFilePath   string `yaml:"kubeConfigFilePath"`

	// The Framework for which the barrier waits.
	FrameworkNamespace string `yaml:"frameworkNamespace"`
	FrameworkName      string `yaml:"frameworkName"`

	// Check interval and timeout to expect all Tasks in the Framework reach the
	// barrier, i.e. are ready with not nil PodIP.
	BarrierCheckIntervalSec int64 `yaml:"barrierCheckIntervalSec"`
	BarrierCheckTimeoutSec  int64 `yaml:"barrierCheckTimeoutSec"`
}

func newConfig() *Config {
	c := Config{}

	// Setting and Defaulting
	c.KubeApiServerAddress = ci.EnvValueKubeApiServerAddress
	if ci.EnvValueKubeConfigFilePath == "" {
		c.KubeConfigFilePath = *defaultKubeConfigFilePath()
	} else {
		c.KubeConfigFilePath = ci.EnvValueKubeConfigFilePath
	}
	c.FrameworkNamespace = os.Getenv(ci.EnvNameFrameworkNamespace)
	c.FrameworkName = os.Getenv(ci.EnvNameFrameworkName)

	barrierCheckIntervalSecStr := os.Getenv(EnvNameBarrierCheckIntervalSec)
	if barrierCheckIntervalSecStr == "" {
		c.BarrierCheckIntervalSec = 10
	} else {
		i, err := strconv.ParseInt(barrierCheckIntervalSecStr, 10, 64)
		if err != nil {
			klog.Errorf(
				"Failed to parse ${%v}: %v",
				EnvNameBarrierCheckIntervalSec, err)
			exit(ci.CompletionCodeContainerPermanentFailed)
		}
		c.BarrierCheckIntervalSec = i
	}

	barrierCheckTimeoutSecStr := os.Getenv(EnvNameBarrierCheckTimeoutSec)
	if barrierCheckTimeoutSecStr == "" {
		c.BarrierCheckTimeoutSec = 10 * 60
	} else {
		i, err := strconv.ParseInt(barrierCheckTimeoutSecStr, 10, 64)
		if err != nil {
			klog.Errorf(
				"Failed to parse ${%v}: %v",
				EnvNameBarrierCheckTimeoutSec, err)
			exit(ci.CompletionCodeContainerPermanentFailed)
		}
		c.BarrierCheckTimeoutSec = i
	}

	// Validation
	errPrefix := "Validation Failed: "
	if c.FrameworkName == "" {
		klog.Errorf(errPrefix+
			"${%v} should not be empty",
			ci.EnvNameFrameworkName)
		exit(ci.CompletionCodeContainerPermanentFailed)
	}
	if c.BarrierCheckIntervalSec < 5 {
		klog.Errorf(errPrefix+
			"${%v} %v should not be less than 5",
			EnvNameBarrierCheckIntervalSec, c.BarrierCheckIntervalSec)
		exit(ci.CompletionCodeContainerPermanentFailed)
	}
	if c.BarrierCheckTimeoutSec < 60 || c.BarrierCheckTimeoutSec > 20*60 {
		klog.Errorf(errPrefix+
			"${%v} %v should not be less than 60 or greater than 20 * 60",
			EnvNameBarrierCheckTimeoutSec, c.BarrierCheckTimeoutSec)
		exit(ci.CompletionCodeContainerPermanentFailed)
	}

	return &c
}

func defaultKubeConfigFilePath() *string {
	configPath := ci.DefaultKubeConfigFilePath
	_, err := os.Stat(configPath)
	if err == nil {
		return &configPath
	}

	configPath = ""
	return &configPath
}

func buildKubeConfig(bConfig *Config) *rest.Config {
	kConfig, err := clientcmd.BuildConfigFromFlags(
		bConfig.KubeApiServerAddress, bConfig.KubeConfigFilePath)
	if err != nil {
		klog.Errorf("Failed to build KubeConfig, please ensure "+
			"${KUBE_APISERVER_ADDRESS} or ${KUBECONFIG} or ${HOME}/.kube/config or "+
			"${KUBERNETES_SERVICE_HOST}:${KUBERNETES_SERVICE_PORT} is valid: "+
			"Error: %v", err)
		exit(ci.CompletionCode(1))
	}
	return kConfig
}

///////////////////////////////////////////////////////////////////////////////////////
// Methods
///////////////////////////////////////////////////////////////////////////////////////
func NewFrameworkBarrier() *FrameworkBarrier {
	klog.Infof("Initializing %v", ComponentName)

	bConfig := newConfig()
	klog.Infof("With Config: \n%v", common.ToYaml(bConfig))
	kConfig := buildKubeConfig(bConfig)
	kClient, fClient, _ := internal.CreateClients(kConfig)

	return &FrameworkBarrier{
		kConfig: kConfig,
		bConfig: bConfig,
		kClient: kClient,
		fClient: fClient,
	}
}

func (b *FrameworkBarrier) Run() {
	klog.Infof("Running %v", ComponentName)

	var f *ci.Framework
	var err error
	var isPassed bool
	var isPermanentErr bool
	wait.PollImmediate(
		common.SecToDuration(&b.bConfig.BarrierCheckIntervalSec),
		common.SecToDuration(&b.bConfig.BarrierCheckTimeoutSec),
		func() (bool, error) {
			f, err = b.fClient.FrameworkcontrollerV1().
				Frameworks(b.bConfig.FrameworkNamespace).
				Get(b.bConfig.FrameworkName, meta.GetOptions{})

			if err == nil {
				isPassed = isBarrierPassed(f)
				return isPassed, nil
			} else {
				klog.Warningf("Failed to get Framework object from ApiServer: %v", err)
				if apiErrors.IsNotFound(err) {
					// Permanent Error: Early Stop
					isPermanentErr = true
					return false, err
				} else {
					// Unknown Error: Poll Until Timeout
					isPermanentErr = false
					return false, nil
				}
			}
		})

	if isPassed {
		klog.Infof("BarrierPassed: " +
			"All Tasks are ready with not nil PodIP.")
		dumpFramework(f)
		generateInjector(f)
		exit(ci.CompletionCodeSucceeded)
	} else {
		if err == nil {
			klog.Errorf("BarrierNotPassed: " +
				"Timeout to wait all Tasks are ready with not nil PodIP.")
			exit(ci.CompletionCodeContainerTransientConflictFailed)
		} else {
			klog.Errorf("Failed to get Framework object from ApiServer: %v", err)
			if isPermanentErr {
				exit(ci.CompletionCodeContainerPermanentFailed)
			} else {
				exit(ci.CompletionCode(1))
			}
		}
	}
}

func isBarrierPassed(f *ci.Framework) bool {
	totalTaskCount := int32(0)
	for _, taskRole := range f.Spec.TaskRoles {
		totalTaskCount += taskRole.TaskNumber
	}

	readyTaskCount := int32(0)
	if f.Status != nil {
		for _, taskRoleStatus := range f.TaskRoleStatuses() {
			for _, taskStatus := range taskRoleStatus.TaskStatuses {
				if isTaskReady(taskStatus) {
					readyTaskCount++
				}
			}
		}
	}

	// Wait until readyTaskCount is consistent with totalTaskCount.
	if readyTaskCount == totalTaskCount {
		klog.Infof("BarrierPassed: "+
			"%v/%v Tasks are ready with not nil PodIP.",
			readyTaskCount, totalTaskCount)
		return true
	} else {
		klog.Warningf("BarrierNotPassed: "+
			"%v/%v Tasks are ready with not nil PodIP.",
			readyTaskCount, totalTaskCount)
		return false
	}
}

func isTaskReady(taskStatus *ci.TaskStatus) bool {
	return taskStatus.AttemptStatus.PodIP != nil &&
		*taskStatus.AttemptStatus.PodIP != ""
}

func dumpFramework(f *ci.Framework) {
	err := ioutil.WriteFile(FrameworkObjectFilePath, []byte(common.ToJson(f)), 0644)
	if err != nil {
		klog.Errorf(
			"Failed to dump the Framework object to local file: %v, %v",
			FrameworkObjectFilePath, err)
		exit(ci.CompletionCode(1))
	}

	klog.Infof(
		"Succeeded to dump the Framework object to local file: %v",
		FrameworkObjectFilePath)
}

func getTaskRoleEnvName(taskRoleName string, suffix string) string {
	return strings.Join([]string{"FB", strings.ToUpper(taskRoleName), suffix}, "_")
}

func generateInjector(f *ci.Framework) {
	var injector strings.Builder
	injector.WriteString("#!/bin/bash")
	injector.WriteString("\n")

	if f.Status != nil {
		injector.WriteString("\n")
		injector.WriteString(
			"echo " + InjectorFilePath + ": Start to inject environment variables")
		injector.WriteString("\n")

		// FB_{UpperCase({TaskRoleName})}_IPS=
		//   {Task[0].PodIP},...,
		//   {Task[TaskRole.TaskNumber-1].PodIP}
		injector.WriteString("\n")
		for _, taskRoleStatus := range f.TaskRoleStatuses() {
			ipsEnvName := getTaskRoleEnvName(taskRoleStatus.Name, "IPS")

			injector.WriteString("export " + ipsEnvName + "=")
			for _, taskStatus := range taskRoleStatus.TaskStatuses {
				taskIndex := taskStatus.Index
				if taskIndex > 0 {
					injector.WriteString(",")
				}
				taskIP := *taskStatus.AttemptStatus.PodIP
				injector.WriteString(taskIP)
			}
			injector.WriteString("\n")
			injector.WriteString("echo " + ipsEnvName + "=${" + ipsEnvName + "}")
			injector.WriteString("\n")
		}

		// FB_{UpperCase({TaskRoleName})}_ADDRESSES=
		//   {Task[0].PodIP}:${FB_{UpperCase({TaskRoleName})}_PORT},...,
		//   {Task[TaskRole.TaskNumber-1].PodIP}:${FB_{UpperCase({TaskRoleName})}_PORT}
		injector.WriteString("\n")
		for _, taskRoleStatus := range f.TaskRoleStatuses() {
			addrsEnvName := getTaskRoleEnvName(taskRoleStatus.Name, "ADDRESSES")
			portEnvName := getTaskRoleEnvName(taskRoleStatus.Name, "PORT")

			injector.WriteString("export " + addrsEnvName + "=")
			for _, taskStatus := range taskRoleStatus.TaskStatuses {
				taskIndex := taskStatus.Index
				if taskIndex > 0 {
					injector.WriteString(",")
				}
				taskAddr := *taskStatus.AttemptStatus.PodIP + ":" + "${" + portEnvName + "}"
				injector.WriteString(taskAddr)
			}
			injector.WriteString("\n")
			injector.WriteString("echo " + addrsEnvName + "=${" + addrsEnvName + "}")
			injector.WriteString("\n")
		}

		injector.WriteString("\n")
		injector.WriteString(
			"echo " + InjectorFilePath + ": Succeeded to inject environment variables")
		injector.WriteString("\n")
	}

	err := ioutil.WriteFile(InjectorFilePath, []byte(injector.String()), 0755)
	if err != nil {
		klog.Errorf(
			"Failed to generate the injector script to local file: %v, %v",
			InjectorFilePath, err)
		exit(ci.CompletionCode(1))
	}

	klog.Infof(
		"Succeeded to generate the injector script to local file: %v",
		InjectorFilePath)
}

func exit(cc ci.CompletionCode) {
	logPfx := fmt.Sprintf("ExitCode: %v: Exit with ", cc)
	if cc == ci.CompletionCodeSucceeded {
		klog.Infof(logPfx + "success.")
	} else if cc == ci.CompletionCodeContainerTransientFailed {
		klog.Errorf(logPfx +
			"transient failure to tell controller to retry.")
	} else if cc == ci.CompletionCodeContainerTransientConflictFailed {
		klog.Errorf(logPfx +
			"transient conflict failure to tell controller to back off retry.")
	} else if cc == ci.CompletionCodeContainerPermanentFailed {
		klog.Errorf(logPfx +
			"permanent failure to tell controller not to retry.")
	} else {
		klog.Errorf(logPfx +
			"unknown failure to tell controller to retry within maxRetryCount.")
	}

	os.Exit(int(cc))
}
