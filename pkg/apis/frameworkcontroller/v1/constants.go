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
	core "k8s.io/api/core/v1"
)

///////////////////////////////////////////////////////////////////////////////////////
// General Constants
///////////////////////////////////////////////////////////////////////////////////////
const (
	// For controller
	ComponentName      = "frameworkcontroller"
	GroupName          = "frameworkcontroller.microsoft.com"
	Version            = "v1"
	FrameworkPlural    = "frameworks"
	FrameworkCRDName   = FrameworkPlural + "." + GroupName
	FrameworkKind      = "Framework"
	ConfigMapKind      = "ConfigMap"
	ObjectUIDFieldPath = "metadata.uid"

	ConfigFilePath         = "./frameworkcontroller.yaml"
	UnlimitedValue         = -1
	ExtendedUnlimitedValue = -2

	// For all managed objects
	// Predefined Annotations
	AnnotationKeyFrameworkNamespace = "FC_FRAMEWORK_NAMESPACE"
	AnnotationKeyFrameworkName      = "FC_FRAMEWORK_NAME"
	AnnotationKeyTaskRoleName       = "FC_TASKROLE_NAME"
	AnnotationKeyTaskIndex          = "FC_TASK_INDEX"
	AnnotationKeyConfigMapName      = "FC_CONFIGMAP_NAME"
	AnnotationKeyPodName            = "FC_POD_NAME"

	AnnotationKeyFrameworkAttemptID          = "FC_FRAMEWORK_ATTEMPT_ID"
	AnnotationKeyFrameworkAttemptInstanceUID = "FC_FRAMEWORK_ATTEMPT_INSTANCE_UID"
	AnnotationKeyConfigMapUID                = "FC_CONFIGMAP_UID"
	AnnotationKeyTaskAttemptID               = "FC_TASK_ATTEMPT_ID"

	// Predefined Labels
	LabelKeyFrameworkName = AnnotationKeyFrameworkName
	LabelKeyTaskRoleName  = AnnotationKeyTaskRoleName

	// For all managed containers
	// Predefined Environment Variables
	// It can be referred by the environment variable specified in the spec, i.e.
	// specify the environment variable value to include "$(AnyPredefinedEnvName)".
	EnvNameFrameworkNamespace = AnnotationKeyFrameworkNamespace
	EnvNameFrameworkName      = AnnotationKeyFrameworkName
	EnvNameTaskRoleName       = AnnotationKeyTaskRoleName
	EnvNameTaskIndex          = AnnotationKeyTaskIndex
	EnvNameConfigMapName      = AnnotationKeyConfigMapName
	EnvNamePodName            = AnnotationKeyPodName

	EnvNameFrameworkAttemptID          = AnnotationKeyFrameworkAttemptID
	EnvNameFrameworkAttemptInstanceUID = AnnotationKeyFrameworkAttemptInstanceUID
	EnvNameConfigMapUID                = AnnotationKeyConfigMapUID
	EnvNameTaskAttemptID               = AnnotationKeyTaskAttemptID
	EnvNameTaskAttemptInstanceUID      = "FC_TASK_ATTEMPT_INSTANCE_UID"
	EnvNamePodUID                      = "FC_POD_UID"
)

var FrameworkGroupVersionKind = SchemeGroupVersion.WithKind(FrameworkKind)
var ConfigMapGroupVersionKind = core.SchemeGroupVersion.WithKind(ConfigMapKind)
var ObjectUIDEnvVarSource = &core.EnvVarSource{
	FieldRef: &core.ObjectFieldSelector{FieldPath: ObjectUIDFieldPath},
}

var EnvValueKubeApiServerAddress = os.Getenv("KUBE_APISERVER_ADDRESS")
var EnvValueKubeConfigFilePath = os.Getenv("KUBECONFIG")
var DefaultKubeConfigFilePath = os.Getenv("HOME") + "/.kube/config"

///////////////////////////////////////////////////////////////////////////////////////
// CompletionCodeInfos
///////////////////////////////////////////////////////////////////////////////////////
type CompletionCodeInfo struct {
	Phrase CompletionPhrase
	Type   CompletionType
}

type ContainerStateTerminatedReason string

const (
	ReasonOOMKilled ContainerStateTerminatedReason = "OOMKilled"
)

// Defined according to the CompletionCode Convention.
// See CompletionStatus.
const (
	// NonNegative:
	// ExitCode of the Framework's Container
	// [129, 165]: Container Received Fatal Error Signal: ExitCode - 128
	CompletionCodeContainerSigTermReceived CompletionCode = 143
	CompletionCodeContainerSigKillReceived CompletionCode = 137
	CompletionCodeContainerSigIntReceived  CompletionCode = 130
	// [200, 219]: Container ExitCode Contract
	CompletionCodeContainerTransientFailed         CompletionCode = 200
	CompletionCodeContainerTransientConflictFailed CompletionCode = 201
	CompletionCodeContainerPermanentFailed         CompletionCode = 210
	// 0: Succeeded
	CompletionCodeSucceeded CompletionCode = 0

	// Negative:
	// ExitCode of the Framework's Predefined Error
	// -1XX: Framework Predefined Transient Error
	CompletionCodeConfigMapExternalDeleted        CompletionCode = -100
	CompletionCodePodExternalDeleted              CompletionCode = -101
	CompletionCodeConfigMapCreationTimeout        CompletionCode = -110
	CompletionCodePodCreationTimeout              CompletionCode = -111
	CompletionCodePodFailedWithoutFailedContainer CompletionCode = -120
	// -2XX: Framework Predefined Permanent Error
	CompletionCodePodSpecInvalid CompletionCode = -200
	// -3XX: Framework Predefined Unknown Error
	CompletionCodeContainerOOMKilled CompletionCode = -300
)

var CompletionCodeInfos = map[CompletionCode]CompletionCodeInfo{
	CompletionCodeContainerSigTermReceived: {
		"ContainerSigTermReceived", CompletionType{CompletionTypeNameFailed,
			[]CompletionTypeAttribute{CompletionTypeAttributeTransient, CompletionTypeAttributeExternal}}},
	CompletionCodeContainerSigKillReceived: {
		"ContainerSigKillReceived", CompletionType{CompletionTypeNameFailed,
			[]CompletionTypeAttribute{CompletionTypeAttributeTransient, CompletionTypeAttributeExternal}}},
	CompletionCodeContainerSigIntReceived: {
		"ContainerSigIntReceived", CompletionType{CompletionTypeNameFailed,
			[]CompletionTypeAttribute{CompletionTypeAttributeTransient, CompletionTypeAttributeExternal}}},
	CompletionCodeContainerTransientFailed: {
		"ContainerTransientFailed", CompletionType{CompletionTypeNameFailed,
			[]CompletionTypeAttribute{CompletionTypeAttributeTransient, CompletionTypeAttributeInternal}}},
	CompletionCodeContainerTransientConflictFailed: {
		"ContainerTransientConflictFailed", CompletionType{CompletionTypeNameFailed,
			[]CompletionTypeAttribute{CompletionTypeAttributeTransient, CompletionTypeAttributeInternal,
				CompletionTypeAttributeConflict}}},
	CompletionCodeContainerPermanentFailed: {
		"ContainerPermanentFailed", CompletionType{CompletionTypeNameFailed,
			[]CompletionTypeAttribute{CompletionTypeAttributePermanent, CompletionTypeAttributeInternal}}},
	CompletionCodeSucceeded: {
		"Succeeded", CompletionType{CompletionTypeNameSucceeded,
			[]CompletionTypeAttribute{CompletionTypeAttributeInternal}}},
	CompletionCodeConfigMapExternalDeleted: {
		"ConfigMapExternalDeleted", CompletionType{CompletionTypeNameFailed,
			[]CompletionTypeAttribute{CompletionTypeAttributeTransient, CompletionTypeAttributeExternal}}},
	CompletionCodePodExternalDeleted: {
		// Possibly be due to Pod Eviction.
		"PodExternalDeleted", CompletionType{CompletionTypeNameFailed,
			[]CompletionTypeAttribute{CompletionTypeAttributeTransient, CompletionTypeAttributeExternal}}},
	CompletionCodeConfigMapCreationTimeout: {
		"ConfigMapCreationTimeout", CompletionType{CompletionTypeNameFailed,
			[]CompletionTypeAttribute{CompletionTypeAttributeTransient, CompletionTypeAttributeExternal}}},
	CompletionCodePodCreationTimeout: {
		"PodCreationTimeout", CompletionType{CompletionTypeNameFailed,
			[]CompletionTypeAttribute{CompletionTypeAttributeTransient, CompletionTypeAttributeExternal}}},
	CompletionCodePodFailedWithoutFailedContainer: {
		"PodFailedWithoutFailedContainer", CompletionType{CompletionTypeNameFailed,
			[]CompletionTypeAttribute{CompletionTypeAttributeTransient, CompletionTypeAttributeExternal}}},
	CompletionCodePodSpecInvalid: {
		"PodSpecInvalid", CompletionType{CompletionTypeNameFailed,
			[]CompletionTypeAttribute{CompletionTypeAttributePermanent, CompletionTypeAttributeInternal}}},
	CompletionCodeContainerOOMKilled: {
		// May be due to exceed the Container memory limit or the Container workload spike or
		// OS memory pressure, so it may be Permanent or Transient, Internal or External.
		"ContainerOOMKilled", CompletionType{CompletionTypeNameFailed,
			[]CompletionTypeAttribute{}}},
}

var CompletionCodeInfoContainerFailedWithUnknownExitCode = CompletionCodeInfo{
	"ContainerFailedWithUnknownExitCode", CompletionType{CompletionTypeNameFailed,
		[]CompletionTypeAttribute{}},
}
