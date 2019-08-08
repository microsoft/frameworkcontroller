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
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type FrameworkList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata"`
	Items         []Framework `json:"items"`
}

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

//////////////////////////////////////////////////////////////////////////////////////////////////
// A Framework represents an application with a set of Tasks:
// 1. Executed by Kubernetes Pod
// 2. Partitioned to different heterogeneous TaskRoles which share the same lifecycle
// 3. Ordered in the same homogeneous TaskRole by TaskIndex
// 4. With consistent identity {FrameworkName}-{TaskRoleName}-{TaskIndex} as PodName
// 5. With fine grained RetryPolicy for each Task and the whole Framework
// 6. With fine grained FrameworkAttemptCompletionPolicy for each TaskRole
// 7. Guarantees at most one instance of a specific Task is running at any point in time
// 8. Guarantees at most one instance of a specific Framework is running at any point in time
//
// Notes:
// 1. Status field should only be modified by FrameworkController, and
//    other fields should not be modified by FrameworkController.
//    TODO: Remove +genclient:noStatus after ApiServer has supported CRD Subresources.
//    Leverage CRD status subresource to isolate Status field modification with other fields.
//    This can help to avoid unintended modification, such as users may unintendedly modify
//    the status when updating the spec.
// 2. To ensure at most one instance of a specific Task is running at any point in time:
//    1. Do not delete the managed Pod with 0 gracePeriodSeconds.
//       For example, the default Pod deletion is acceptable.
//    2. Do not delete the Node which runs the managed Pod.
//       For example, drain before delete the Node is acceptable.
//    The instance can be universally located by its TaskAttemptInstanceUID or PodUID.
//    See RetryPolicySpec and TaskAttemptStatus.
// 3. To ensure at most one instance of a specific Framework is running at any point in time:
//    1. Ensure ensure at most one instance of a specific Task is running at any point in time.
//    2. Do not delete the managed ConfigMap with Background propagationPolicy.
//       For example, the default ConfigMap deletion is acceptable.
//    3. Must delete the Framework with Foreground propagationPolicy.
//       For example, the default Framework deletion may not be acceptable, since the default
//       propagationPolicy for Framework object may be Background.
//    The instance can be universally located by its FrameworkAttemptInstanceUID or ConfigMapUID.
//    See RetryPolicySpec and FrameworkAttemptStatus.
// 4. To ensure there is no orphan object previously managed by FrameworkController:
//    1. Do not delete the Framework or the managed ConfigMap with Orphan propagationPolicy.
//       For example, the default Framework and ConfigMap deletion is acceptable.
//    2. Do not change the OwnerReferences of the managed ConfigMap and Pods.
//////////////////////////////////////////////////////////////////////////////////////////////////
type Framework struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata"`
	Spec            FrameworkSpec    `json:"spec"`
	Status          *FrameworkStatus `json:"status"`
}

//////////////////////////////////////////////////////////////////////////////////////////////////
// Spec
//////////////////////////////////////////////////////////////////////////////////////////////////
type FrameworkSpec struct {
	Description string `json:"description"`
	// Only support to update from ExecutionStart to ExecutionStop
	ExecutionType ExecutionType   `json:"executionType"`
	RetryPolicy   RetryPolicySpec `json:"retryPolicy"`
	TaskRoles     []*TaskRoleSpec `json:"taskRoles"`
}

type TaskRoleSpec struct {
	// TaskRoleName
	Name string `json:"name"`

	// Tasks with TaskIndex in range [0, TaskNumber)
	TaskNumber                       int32                `json:"taskNumber"`
	FrameworkAttemptCompletionPolicy CompletionPolicySpec `json:"frameworkAttemptCompletionPolicy"`
	Task                             TaskSpec             `json:"task"`
}

type TaskSpec struct {
	RetryPolicy RetryPolicySpec      `json:"retryPolicy"`
	Pod         core.PodTemplateSpec `json:"pod"`
}

type ExecutionType string

const (
	ExecutionStart ExecutionType = "Start"
	ExecutionStop  ExecutionType = "Stop"
)

// RetryPolicySpec can be configured for the whole Framework and each TaskRole
// to control:
// 1. Framework RetryPolicy:
//    The conditions to retry the whole Framework after the Framework's current
//    FrameworkAttempt completed.
//    It can also be considered as Framework CompletionPolicy, i.e. the conditions
//    to complete the whole Framework.
// 2. Task RetryPolicy:
//    The conditions to retry a single Task in the TaskRole after the Task's
//    current TaskAttempt completed.
//    It can also be considered as Task CompletionPolicy, i.e. the conditions to
//    complete a single Task in the TaskRole.
//
// Usage:
// If the Pod Spec is invalid or
// the ExecutionType is ExecutionStop or
// the Task's FrameworkAttempt is completing,
//   will not retry.
//
// If the FancyRetryPolicy is enabled,
//   will retry if the completion is due to Transient Failed CompletionType,
//   will not retry if the completion is due to Permanent Failed CompletionType,
//   will apply the NormalRetryPolicy defined below if all above conditions are
//   not satisfied.
//
// If the FancyRetryPolicy is not enabled,
//   will directly apply the NormalRetryPolicy for all kinds of completions.
//
// The NormalRetryPolicy is defined as,
//   will retry and AccountableRetriedCount++ if MaxRetryCount == -2,
//   will retry and AccountableRetriedCount++ if the completion is due to any
//     failure and MaxRetryCount == -1,
//   will retry and AccountableRetriedCount++ if the completion is due to any
//     failure and AccountableRetriedCount < MaxRetryCount,
//   will not retry if all above conditions are not satisfied.
//
// After the retry is exhausted, the final CompletionStatus is defined as,
//   the CompletionStatus of the last attempt.
//
// Notes:
// 1. The existence of an attempt instance may not always be observed, such as
//    create fails but succeeds on remote and then followed by an external delete.
//    So, an attempt identified by its attempt id may be associated with multiple
//    attempt instances over time, i.e. multiple instances may be run for the
//    attempt over time, however, at most one instance is exposed into ApiServer
//    over time and at most one instance is running at any point in time.
//    So, the actual retried attempt instances maybe exceed the RetryPolicySpec
//    in rare cases, however, the RetryPolicyStatus will never exceed the
//    RetryPolicySpec.
// 2. Resort to other spec to control other kind of RetryPolicy:
//    1. Container RetryPolicy is the RestartPolicy in Pod Spec.
//       See https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#restart-policy
type RetryPolicySpec struct {
	FancyRetryPolicy bool  `json:"fancyRetryPolicy"`
	MaxRetryCount    int32 `json:"maxRetryCount"`
}

// CompletionPolicySpec can be configured for each TaskRole to control:
// 1. FrameworkAttempt CompletionPolicy:
//    1. The conditions to complete a FrameworkAttempt.
//    2. The CompletionStatus of the completed FrameworkAttempt.
//
// Usage:
// 1. If the ExecutionType is ExecutionStop, immediately complete the FrameworkAttempt,
//    regardless of any uncompleted Task, and the CompletionStatus is failed which
//    is not generated from any Task.
// 2. If MinFailedTaskCount != -1 and MinFailedTaskCount <= failed Task count of
//    current TaskRole, immediately complete the FrameworkAttempt, regardless of
//    any uncompleted Task, and the CompletionStatus is failed which is generated
//    from the Task which triggers the completion.
// 3. If MinSucceededTaskCount != -1 and MinSucceededTaskCount <= succeeded Task
//    count of current TaskRole, immediately complete the FrameworkAttempt, regardless
//    of any uncompleted Task, and the CompletionStatus is succeeded which is
//    generated from the Task which triggers the completion.
// 4. If multiple above 1. and 2. conditions of all TaskRoles are satisfied at the
//    same time, the behavior can be any one of these satisfied conditions.
// 5. If none of above 1. and 2. conditions of all TaskRoles are satisfied until all
//    Tasks of the Framework are completed, immediately complete the FrameworkAttempt
//    and the CompletionStatus is succeeded which is not generated from any Task.
//
// Notes:
// 1. When the FrameworkAttempt is completed, the FrameworkState is transitioned to
//    FrameworkAttemptCompleted, so the Framework may still be retried with another
//    new FrameworkAttempt according to the Framework RetryPolicySpec.
// 2. Resort to other spec to control other kind of CompletionPolicy:
//    1. Framework CompletionPolicy is equivalent to Framework RetryPolicy.
//    2. Task CompletionPolicy is equivalent to Task RetryPolicy.
//    3. TaskAttempt CompletionPolicy is equivalent to Pod CompletionPolicy,
//       i.e. the PodPhase conditions for PodSucceeded or PodFailed.
//       See https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-phase
type CompletionPolicySpec struct {
	MinFailedTaskCount    int32 `json:"minFailedTaskCount"`
	MinSucceededTaskCount int32 `json:"minSucceededTaskCount"`
}

//////////////////////////////////////////////////////////////////////////////////////////////////
// Status
// It is used to:
// 1. Aggregate the ground truth from other related objects, such as Pod.Status.
// 2. Maintain the Framework owned ground truth, such as PodUID.
// 3. Retain the ground truth even if other related objects are deleted.
//
// Notes:
// 1. It should only contain current status, history status should be a different type
//    and stored in a history database.
// 2. For field which is not the ground truth, such as the TaskState, it should be
//    totally reconstructable from its ground truth, in case the Status is failed to
//    persist due to FrameworkController restart.
//    The ground truth may be other fields in Framework.Status or the fields in other
//    related objects, such as the PodUID and Pod.Status.
// 3. For field which is the ground truth, such as the PodUID, it should be
//    Monotonically Exposed which means it should only be changed to a future state in
//    ApiServer. However, it does not mean other related objects are also Monotonically
//    Exposed.
//    For example, from the view of any ApiServer client, the PodUID should be changed
//    from a not nil value to a different not nil value, if and only if its TaskAttemptID
//    is also increased.
// 4. It is better to keep the ground truth in other related objects instead of in the
//    Status here, so that the Framework can be more compatible with other k8s features,
//    such as labels and selectors.
//////////////////////////////////////////////////////////////////////////////////////////////////
type FrameworkStatus struct {
	StartTime         meta.Time              `json:"startTime"`
	CompletionTime    *meta.Time             `json:"completionTime"`
	State             FrameworkState         `json:"state"`
	TransitionTime    meta.Time              `json:"transitionTime"`
	RetryPolicyStatus RetryPolicyStatus      `json:"retryPolicyStatus"`
	AttemptStatus     FrameworkAttemptStatus `json:"attemptStatus"`
}

type FrameworkAttemptStatus struct {
	// FrameworkAttemptID = {FrameworkStatus.RetryPolicyStatus.TotalRetriedCount}
	// It can only locate the FrameworkAttempt within a specific Framework, i.e.
	// it cannot universally locate the FrameworkAttempt and cannot locate the
	// FrameworkAttemptInstance even within a specific Framework.
	ID int32 `json:"id"`

	StartTime      meta.Time  `json:"startTime"`
	RunTime        *meta.Time `json:"runTime"`
	CompletionTime *meta.Time `json:"completionTime"`

	// Current associated FrameworkAttemptInstance:
	// FrameworkAttemptInstanceUID = {FrameworkAttemptID}_{ConfigMapUID}
	// It is ordered by FrameworkAttemptID and can universally locate the
	// FrameworkAttemptInstance.
	InstanceUID *types.UID `json:"instanceUID"`
	// A FrameworkAttemptInstance is represented by a ConfigMap object:
	// ConfigMapName = {FrameworkName}-attempt
	// It will never be changed during the whole lifetime of a specific Framework.
	ConfigMapName string `json:"configMapName"`
	// ConfigMapUID can also universally locate the FrameworkAttemptInstance.
	ConfigMapUID     *types.UID        `json:"configMapUID"`
	CompletionStatus *CompletionStatus `json:"completionStatus"`
	TaskRoleStatuses []*TaskRoleStatus `json:"taskRoleStatuses"`
}

type TaskRoleStatus struct {
	// TaskRoleName
	Name string `json:"name"`

	// Tasks with TaskIndex in range [0, TaskNumber)
	TaskStatuses []*TaskStatus `json:"taskStatuses"`
}

type TaskStatus struct {
	// TaskIndex
	Index int32 `json:"index"`

	StartTime         meta.Time         `json:"startTime"`
	CompletionTime    *meta.Time        `json:"completionTime"`
	State             TaskState         `json:"state"`
	TransitionTime    meta.Time         `json:"transitionTime"`
	RetryPolicyStatus RetryPolicyStatus `json:"retryPolicyStatus"`
	AttemptStatus     TaskAttemptStatus `json:"attemptStatus"`
}

type TaskAttemptStatus struct {
	// TaskAttemptID = {TaskStatus.RetryPolicyStatus.TotalRetriedCount}
	// It can only locate the TaskAttempt within a specific Task, i.e. it cannot
	// universally locate the TaskAttempt and cannot locate the TaskAttemptInstance
	// even within a specific Task.
	ID int32 `json:"id"`

	StartTime      meta.Time  `json:"startTime"`
	RunTime        *meta.Time `json:"runTime"`
	CompletionTime *meta.Time `json:"completionTime"`

	// Current associated TaskAttemptInstance:
	// TaskAttemptInstanceUID = {TaskAttemptID}_{PodUID}
	// It is ordered by TaskAttemptID and can universally locate the
	// TaskAttemptInstance.
	InstanceUID *types.UID `json:"instanceUID"`
	// A TaskAttemptInstance is represented by a Pod object:
	// PodName = {FrameworkName}-{TaskRoleName}-{TaskIndex}
	// It will never be changed during the whole lifetime of a specific Task.
	PodName string `json:"podName"`
	// PodUID can also universally locate the TaskAttemptInstance.
	PodUID           *types.UID        `json:"podUID"`
	PodIP            *string           `json:"podIP"`
	PodHostIP        *string           `json:"podHostIP"`
	CompletionStatus *CompletionStatus `json:"completionStatus"`
}

type RetryPolicyStatus struct {
	// Used as the ground truth of current attempt id.
	// If it is for Framework, TotalRetriedCount = FrameworkAttemptID
	// If it is for Task, TotalRetriedCount = TaskAttemptID
	TotalRetriedCount int32 `json:"totalRetriedCount"`

	// Used to compare against MaxRetryCount.
	// If the FancyRetryPolicy is not enabled,
	//   it is the same as the TotalRetriedCount.
	// If the FancyRetryPolicy is enabled,
	//   it does not count into the retries for the completion which is due to
	//   Transient CompletionType, so only in this case, it may be less than the
	//   TotalRetriedCount.
	AccountableRetriedCount int32 `json:"accountableRetriedCount"`

	// Used to expose the ScheduledRetryTime after which current retry can be
	// executed.
	// ScheduledRetryTime = AttemptStatus.CompletionTime + RetryDelaySec
	// It is available and meaningful if and only if current attempt is in
	// AttemptCompleted state.
	RetryDelaySec *int64 `json:"retryDelaySec"`
}

type CompletionStatus struct {
	// CompletionCode Convention:
	// 1. NonNegative:
	//    The CompletionCode is the ExitCode of the Framework's Container which
	//    triggers the completion.
	// 2. Negative:
	//    -1XX: Framework Predefined Transient Error
	//    -2XX: Framework Predefined Permanent Error
	//    -3XX: Framework Predefined Unknown Error
	//    The CompletionCode is the ExitCode of the Framework's Predefined Error
	//    which triggers the completion.
	Code CompletionCode `json:"code"`
	// The textual phrase representation of the CompletionCode.
	Phrase CompletionPhrase `json:"phrase"`

	// CompletionType is determined by the CompletionCode and the Predefined
	// CompletionCodeInfos.
	// See CompletionCodeInfos.
	Type CompletionType `json:"type"`

	// The detailed diagnostic information of the completion.
	Diagnostics string `json:"diagnostics"`
}

type CompletionCode int32

type CompletionPhrase string

type CompletionType struct {
	Name       CompletionTypeName        `json:"name"`
	Attributes []CompletionTypeAttribute `json:"attributes"`
}

type CompletionTypeName string

const (
	CompletionTypeNameSucceeded CompletionTypeName = "Succeeded"
	CompletionTypeNameFailed    CompletionTypeName = "Failed"
)

type CompletionTypeAttribute string

const (
	// CompletionTypeName must be different within a finite retry times:
	// such as failed due to dependent components shutdown, machine error,
	// network error, environment error, workload spike, etc.
	CompletionTypeAttributeTransient CompletionTypeAttribute = "Transient"
	// CompletionTypeName must be the same in every retry times:
	// such as failed due to incorrect usage, incorrect configuration, etc.
	CompletionTypeAttributePermanent CompletionTypeAttribute = "Permanent"

	// The completion must be caused by the Platform,
	// such as failed due to the instability of FrameworkController, K8S, OS,
	// machine, network, etc.
	CompletionTypeAttributePlatform CompletionTypeAttribute = "Platform"
	// The completion must be caused by the user of the Framework,
	// such as failed due to user code bugs, user stop request, etc.
	CompletionTypeAttributeUser CompletionTypeAttribute = "User"

	// The completion must be caused by Resource Conflict (Resource Contention):
	// such as failed due to Gang Allocation timeout.
	CompletionTypeAttributeConflict CompletionTypeAttribute = "Conflict"
)

// The ground truth of FrameworkState is the current associated FrameworkAttemptInstance
// which is represented by the ConfigMapUID and the corresponding ConfigMap object in
// the local cache.
//
// [AssociatedState]: ConfigMapUID is not nil
type FrameworkState string

const (
	// ConfigMap does not exist and
	// may not have been creation requested successfully and is expected to exist.
	// [StartState]
	// [AttemptStartState]
	// -> FrameworkAttemptCreationRequested
	// -> FrameworkAttemptCompleted
	FrameworkAttemptCreationPending FrameworkState = "AttemptCreationPending"

	// ConfigMap does not exist and
	// must have been creation requested successfully and is expected to exist.
	// [AssociatedState]
	// -> FrameworkAttemptPreparing
	// -> FrameworkAttemptDeleting
	// -> FrameworkAttemptCompleted
	FrameworkAttemptCreationRequested FrameworkState = "AttemptCreationRequested"

	// ConfigMap exists and is not deleting and
	// may not have been deletion requested successfully and
	// FrameworkAttemptCompletionPolicy may not have been satisfied and
	// no Task of current attempt has ever entered TaskAttemptRunning state.
	// [AssociatedState]
	// -> FrameworkAttemptRunning
	// -> FrameworkAttemptDeletionPending
	// -> FrameworkAttemptDeleting
	// -> FrameworkAttemptCompleted
	FrameworkAttemptPreparing FrameworkState = "AttemptPreparing"

	// ConfigMap exists and is not deleting and
	// may not have been deletion requested successfully and
	// FrameworkAttemptCompletionPolicy may not have been satisfied and
	// at least one Task of current attempt has ever entered TaskAttemptRunning state.
	// [AssociatedState]
	// -> FrameworkAttemptDeletionPending
	// -> FrameworkAttemptDeleting
	// -> FrameworkAttemptCompleted
	FrameworkAttemptRunning FrameworkState = "AttemptRunning"

	// ConfigMap exists and is not deleting and
	// may not have been deletion requested successfully and
	// FrameworkAttemptCompletionPolicy must have been satisfied.
	// [AssociatedState]
	// -> FrameworkAttemptDeletionRequested
	// -> FrameworkAttemptDeleting
	// -> FrameworkAttemptCompleted
	FrameworkAttemptDeletionPending FrameworkState = "AttemptDeletionPending"

	// ConfigMap exists and is not deleting and
	// must have been deletion requested successfully.
	// [AssociatedState]
	// -> FrameworkAttemptDeleting
	// -> FrameworkAttemptCompleted
	FrameworkAttemptDeletionRequested FrameworkState = "AttemptDeletionRequested"

	// ConfigMap exists and is deleting.
	// [AssociatedState]
	// -> FrameworkAttemptCompleted
	FrameworkAttemptDeleting FrameworkState = "AttemptDeleting"

	// ConfigMap does not exist and
	// is not expected to exist and will never exist and
	// current attempt is not the last attempt or to be determined.
	// [AttemptFinalState]
	// -> FrameworkAttemptCreationPending
	// -> FrameworkCompleted
	FrameworkAttemptCompleted FrameworkState = "AttemptCompleted"

	// ConfigMap does not exist and
	// is not expected to exist and will never exist and
	// current attempt is the last attempt.
	// [FinalState]
	FrameworkCompleted FrameworkState = "Completed"
)

// The ground truth of TaskState is the current associated TaskAttemptInstance
// which is represented by the PodUID and the corresponding Pod object in the
// local cache.
//
// [AssociatedState]: PodUID is not nil
type TaskState string

const (
	// Pod does not exist and
	// may not have been creation requested successfully and is expected to exist.
	// [StartState]
	// [AttemptStartState]
	// -> TaskAttemptCreationRequested
	// -> TaskAttemptCompleted
	TaskAttemptCreationPending TaskState = "AttemptCreationPending"

	// Pod does not exist and
	// must have been creation requested successfully and is expected to exist.
	// [AssociatedState]
	// -> TaskAttemptPreparing
	// -> TaskAttemptDeleting
	// -> TaskAttemptCompleted
	TaskAttemptCreationRequested TaskState = "AttemptCreationRequested"

	// Pod exists and is not deleting and
	// may not have been deletion requested successfully and
	// its PodPhase is PodPending or PodUnknown afterwards.
	// [AssociatedState]
	// -> TaskAttemptRunning
	// -> TaskAttemptDeletionPending
	// -> TaskAttemptDeleting
	// -> TaskAttemptCompleted
	TaskAttemptPreparing TaskState = "AttemptPreparing"

	// Pod exists and is not deleting and
	// may not have been deletion requested successfully and
	// its PodPhase is PodRunning or PodUnknown afterwards.
	// [AssociatedState]
	// -> TaskAttemptDeletionPending
	// -> TaskAttemptDeleting
	// -> TaskAttemptCompleted
	TaskAttemptRunning TaskState = "AttemptRunning"

	// Pod exists and is not deleting and
	// may not have been deletion requested successfully and
	// its PodPhase is PodSucceeded or PodFailed.
	// [AssociatedState]
	// -> TaskAttemptDeletionRequested
	// -> TaskAttemptDeleting
	// -> TaskAttemptCompleted
	TaskAttemptDeletionPending TaskState = "AttemptDeletionPending"

	// Pod exists and is not deleting and
	// must have been deletion requested successfully.
	// [AssociatedState]
	// -> TaskAttemptDeleting
	// -> TaskAttemptCompleted
	TaskAttemptDeletionRequested TaskState = "AttemptDeletionRequested"

	// Pod exists and is deleting.
	// [AssociatedState]
	// -> TaskAttemptCompleted
	TaskAttemptDeleting TaskState = "AttemptDeleting"

	// Pod does not exist and
	// is not expected to exist and will never exist and
	// current attempt is not the last attempt or to be determined.
	// [AttemptFinalState]
	// -> TaskAttemptCreationPending
	// -> TaskCompleted
	TaskAttemptCompleted TaskState = "AttemptCompleted"

	// Pod does not exist and
	// is not expected to exist and will never exist and
	// current attempt is the last attempt.
	// [FinalState]
	TaskCompleted TaskState = "Completed"
)
