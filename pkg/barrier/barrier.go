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
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	ci "github.com/microsoft/frameworkcontroller/pkg/apis/frameworkcontroller/v1"
	frameworkClient "github.com/microsoft/frameworkcontroller/pkg/client/clientset/versioned"
	"github.com/microsoft/frameworkcontroller/pkg/common"
	"github.com/microsoft/frameworkcontroller/pkg/internal"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeClient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
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
	kClient, fClient := internal.CreateClients(kConfig)

	return &FrameworkBarrier{
		kConfig: kConfig,
		bConfig: bConfig,
		kClient: kClient,
		fClient: fClient,
	}
}

func (b *FrameworkBarrier) Run() {
	klog.Infof("Running %v", ComponentName)
	ctx := context.TODO()

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
				Get(ctx, b.bConfig.FrameworkName, meta.GetOptions{})

			if err == nil {
				err = f.Decompress()
				if err == nil {
					isPassed = isBarrierPassed(f)
					return isPassed, nil
				} else {
					klog.Warningf("Failed to decompress Framework object: %v", err)
					// Unknown Error: Poll Until Timeout
					isPermanentErr = false
					return false, nil
				}
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
		klog.Infof("BarrierSucceeded: " +
			"All Tasks are ready with not nil PodIP.")
		dumpFramework(f)
		generateInjector(f)
		exit(ci.CompletionCodeSucceeded)
	} else {
		if err == nil {
			klog.Errorf("BarrierTransientConflictFailed: " +
				"Timeout to wait all Tasks are ready with not nil PodIP.")
			exit(ci.CompletionCodeContainerTransientConflictFailed)
		} else {
			if isPermanentErr {
				klog.Errorf("BarrierPermanentFailed: %v", err)
				exit(ci.CompletionCodeContainerPermanentFailed)
			} else {
				// May also timeout, but still treat as Unknown Error
				klog.Errorf("BarrierUnknownFailed: %v", err)
				exit(ci.CompletionCode(1))
			}
		}
	}
}

func isBarrierPassed(f *ci.Framework) bool {
	// Fully counting Tasks in f.Status against f.Spec, as FrameworkController may
	// have not persist DeletionPending (ScaleDown) Tasks according to current
	// f.Spec.
	totalTaskCount := int32(0)
	readyTaskCount := int32(0)
	for _, taskRoleSpec := range f.Spec.TaskRoles {
		taskRoleName := taskRoleSpec.Name
		taskCountSpec := taskRoleSpec.TaskNumber
		totalTaskCount += taskCountSpec

		if f.Status == nil {
			continue
		}

		taskRoleStatus := f.GetTaskRoleStatus(taskRoleName)
		if taskRoleStatus == nil {
			continue
		}

		taskCountStatus := int32(len(taskRoleStatus.TaskStatuses))
		taskCountStatusAndSpec := common.MinInt32(taskCountStatus, taskCountSpec)
		for taskIndex := int32(0); taskIndex < taskCountStatusAndSpec; taskIndex++ {
			taskStatus := taskRoleStatus.TaskStatuses[taskIndex]
			if isTaskReady(taskStatus, true) {
				readyTaskCount++
			}
		}
	}

	// Wait until readyTaskCount is consistent with totalTaskCount.
	if readyTaskCount >= totalTaskCount {
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

func isTaskReady(ts *ci.TaskStatus, ignoreDeletionPending bool) bool {
	if ts.IsDeletionPendingIgnored(ignoreDeletionPending) {
		return false
	}
	return ts.AttemptStatus.PodIP != nil && *ts.AttemptStatus.PodIP != ""
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

// All Tasks in f.Spec must be also included in f.Status as Ready, so inject from
// f.Status is enough.
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
			taskRoleName := taskRoleStatus.Name
			taskCountStatus := int32(len(taskRoleStatus.TaskStatuses))

			taskRoleSpec := f.GetTaskRoleSpec(taskRoleName)
			if taskRoleSpec == nil {
				continue
			}

			ipsEnvName := getTaskRoleEnvName(taskRoleName, "IPS")
			injector.WriteString("export " + ipsEnvName + "=")

			taskCountSpec := taskRoleSpec.TaskNumber
			taskCountStatusAndSpec := common.MinInt32(taskCountStatus, taskCountSpec)
			for taskIndex := int32(0); taskIndex < taskCountStatusAndSpec; taskIndex++ {
				taskStatus := taskRoleStatus.TaskStatuses[taskIndex]
				taskIP := *taskStatus.AttemptStatus.PodIP
				if taskIndex > 0 {
					injector.WriteString(",")
				}
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
			taskRoleName := taskRoleStatus.Name
			taskCountStatus := int32(len(taskRoleStatus.TaskStatuses))

			taskRoleSpec := f.GetTaskRoleSpec(taskRoleName)
			if taskRoleSpec == nil {
				continue
			}

			addrsEnvName := getTaskRoleEnvName(taskRoleName, "ADDRESSES")
			portEnvName := getTaskRoleEnvName(taskRoleName, "PORT")
			injector.WriteString("export " + addrsEnvName + "=")

			taskCountSpec := taskRoleSpec.TaskNumber
			taskCountStatusAndSpec := common.MinInt32(taskCountStatus, taskCountSpec)
			for taskIndex := int32(0); taskIndex < taskCountStatusAndSpec; taskIndex++ {
				taskStatus := taskRoleStatus.TaskStatuses[taskIndex]
				taskAddr := *taskStatus.AttemptStatus.PodIP + ":" + "${" + portEnvName + "}"
				if taskIndex > 0 {
					injector.WriteString(",")
				}
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
