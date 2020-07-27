# <a name="UserManual">User Manual</a>

## <a name="Index">Index</a>
   - [Framework Interop](#FrameworkInterop)
   - [Container EnvironmentVariable](#ContainerEnvironmentVariable)
   - [Pod Failure Classification](#PodFailureClassification)
   - [Predefined CompletionCode](#PredefinedCompletionCode)
   - [CompletionStatus](#CompletionStatus)
   - [RetryPolicy](#RetryPolicy)
   - [FrameworkAttemptCompletionPolicy](#FrameworkAttemptCompletionPolicy)
   - [Framework ScaleUp/ScaleDown](#FrameworkRescale)
   - [Large Scale Framework](#LargeScaleFramework)
   - [Framework and Pod History](#FrameworkPodHistory)
   - [Framework and Task State Machine](#FrameworkTaskStateMachine)
   - [Framework Consistency vs Availability](#FrameworkConsistencyAvailability)
   - [Controller Extension](#ControllerExtension)
     - [FrameworkBarrier](#FrameworkBarrier)
     - [HiveDScheduler](#HiveDScheduler)
   - [Best Practice](#BestPractice)

## <a name="FrameworkInterop">Framework Interop</a>
### <a name="SupportedClient">Supported Client</a>
As Framework is actually a [Kubernetes CRD](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#customresourcedefinitions), all [CRD Clients](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#accessing-a-custom-resource) can be used to interoperate with it, such as:
1. [kubectl](https://kubernetes.io/docs/reference/kubectl)
   ```shell
   kubectl create -f {Framework File Path}
   # Note this is not Foreground Deletion, see [DELETE Framework] section
   kubectl delete framework {FrameworkName}
   kubectl get framework {FrameworkName}
   kubectl describe framework {FrameworkName}
   kubectl get frameworks
   kubectl describe frameworks
   ...
   ```
2. [Kubernetes Client Library](https://kubernetes.io/docs/reference/using-api/client-libraries)
3. Any HTTP Client

### <a name="SupportedInteroperation">Supported Interoperation</a>
| API Kind | Operations |
|:---- |:---- |
| Framework | [CREATE](#CREATE_Framework) [DELETE](#DELETE_Framework) [GET](#GET_Framework) [LIST](#LIST_Frameworks) [WATCH](#WATCH_Framework) [WATCH_LIST](#WATCH_LIST_Frameworks)<br>[PATCH](#PATCH_Framework) ([Stop](#Stop_Framework), [Add TaskRole](#Add_TaskRole), [Delete TaskRole](#Delete_TaskRole), [Add/Delete Task](#Add_Delete_Task)) |
| [ConfigMap](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#configmap-v1-core) | All operations except for [CREATE](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#create-configmap-v1-core) [PUT](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#replace-configmap-v1-core) [PATCH](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#patch-configmap-v1-core) |
| [Pod](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#pod-v1-core) | All operations except for [CREATE](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#create-pod-v1-core) [PUT](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#replace-pod-v1-core) [PATCH](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#patch-pod-v1-core) |

#### <a name="CREATE_Framework">CREATE Framework</a>
**Request**

    POST /apis/frameworkcontroller.microsoft.com/v1/namespaces/{FrameworkNamespace}/frameworks

Body: [Framework](../pkg/apis/frameworkcontroller/v1/types.go)

Type: application/json or application/yaml

**Description**

Create the specified Framework.

**Response**

| Code | Body | Description |
|:---- |:---- |:---- |
| OK(200) | [Framework](../pkg/apis/frameworkcontroller/v1/types.go) | Return current Framework. |
| Created(201) | [Framework](../pkg/apis/frameworkcontroller/v1/types.go) | Return current Framework. |
| Accepted(202) | [Framework](../pkg/apis/frameworkcontroller/v1/types.go) | Return current Framework. |
| Conflict(409) | [Status](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#status-v1-meta) | The specified Framework already exists. |

#### <a name="PATCH_Framework">PATCH Framework</a>
##### <a name="Stop_Framework">Stop Framework</a>
**Request**

    PATCH /apis/frameworkcontroller.microsoft.com/v1/namespaces/{FrameworkNamespace}/frameworks/{FrameworkName}

Body:

```json
[
  {
    "op": "replace",
    "path": "/spec/executionType",
    "value": "Stop"
  }
]
```

Type: application/json-patch+json

**Description**

Stop the specified Framework:

All running containers of the Framework will be stopped while the object of the Framework is still kept.

**Response**

| Code | Body | Description |
|:---- |:---- |:---- |
| OK(200) | [Framework](../pkg/apis/frameworkcontroller/v1/types.go) | Return current Framework. |
| NotFound(404) | [Status](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#status-v1-meta) | The specified Framework is not found. |

##### <a name="Add_TaskRole">Add TaskRole</a>
**Request**

    PATCH /apis/frameworkcontroller.microsoft.com/v1/namespaces/{FrameworkNamespace}/frameworks/{FrameworkName}

Body:

*Follow the [TaskRoleSpec](../pkg/apis/frameworkcontroller/v1/types.go) to override below $(TaskRoleSpec) placeholder.*

```json
[
  {
    "op": "add",
    "path": "/spec/taskRoles/-",
    "value": $(TaskRoleSpec)
  }
]
```

Type: application/json-patch+json

**Description**

Append the specified TaskRole to the specified Framework.

See more in [Framework ScaleUp/ScaleDown](#FrameworkRescale).

**Response**

| Code | Body | Description |
|:---- |:---- |:---- |
| OK(200) | [Framework](../pkg/apis/frameworkcontroller/v1/types.go) | Return current Framework. |
| NotFound(404) | [Status](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#status-v1-meta) | The specified Framework is not found. |

##### <a name="Delete_TaskRole">Delete TaskRole</a>
**Request**

    PATCH /apis/frameworkcontroller.microsoft.com/v1/namespaces/{FrameworkNamespace}/frameworks/{FrameworkName}

Body:

*Use the current array index of the $(TaskRoleName) to override below $(TaskRoleIndex).*

```json
[
  {
    "op": "test",
    "path": "/spec/taskRoles/$(TaskRoleIndex)/name",
    "value": "$(TaskRoleName)"
  },
  {
    "op": "remove",
    "path": "/spec/taskRoles/$(TaskRoleIndex)"
  }
]
```

Type: application/json-patch+json

**Description**

Delete the specified TaskRole from the specified Framework.

See more in [Framework ScaleUp/ScaleDown](#FrameworkRescale).

Notes:
* Better to first delete all Tasks in the TaskRole and wait until they are all deleted, then delete the whole TaskRole. Otherwise, the Tasks in the TaskRole will be deleted according to last observed [PodGracefulDeletionTimeoutSec](../pkg/apis/frameworkcontroller/v1/types.go) in TaskRoleStatus, as the [PodGracefulDeletionTimeoutSec](../pkg/apis/frameworkcontroller/v1/types.go) in TaskSpec is already deleted.

**Response**

| Code | Body | Description |
|:---- |:---- |:---- |
| OK(200) | [Framework](../pkg/apis/frameworkcontroller/v1/types.go) | Return current Framework. |
| NotFound(404) | [Status](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#status-v1-meta) | The specified Framework is not found. |
| UnprocessableEntity(422) | [Status](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#status-v1-meta) | The specified $(TaskRoleName) does not exist or does not match the specified $(TaskRoleIndex). |

##### <a name="Add_Delete_Task">Add/Delete Task</a>
**Request**

    PATCH /apis/frameworkcontroller.microsoft.com/v1/namespaces/{FrameworkNamespace}/frameworks/{FrameworkName}

Body:

*Use the current array index of the $(TaskRoleName) to override below $(TaskRoleIndex).*

```json
[
  {
    "op": "test",
    "path": "/spec/taskRoles/$(TaskRoleIndex)/name",
    "value": "$(TaskRoleName)"
  },
  {
    "op": "replace",
    "path": "/spec/taskRoles/$(TaskRoleIndex)/taskNumber",
    "value": $(TaskNumber)
  }
]
```

*Generally, you may also need to adjust the TaskRole's [FrameworkAttemptCompletionPolicy](#FrameworkAttemptCompletionPolicy) according to the new $(TaskNumber). It is safe as [Framework ScaleUp/ScaleDown Strong Safety Guarantee](#FrameworkRescaleGuarantee).*

```json
[
  {
    "op": "test",
    "path": "/spec/taskRoles/$(TaskRoleIndex)/name",
    "value": "$(TaskRoleName)"
  },
  {
    "op": "replace",
    "path": "/spec/taskRoles/$(TaskRoleIndex)/taskNumber",
    "value": $(TaskNumber)
  },
  {
    "op": "replace",
    "path": "/spec/taskRoles/$(TaskRoleIndex)/frameworkAttemptCompletionPolicy/minSucceededTaskCount",
    "value": $(MinSucceededTaskCount)
  },
  {
    "op": "replace",
    "path": "/spec/taskRoles/$(TaskRoleIndex)/frameworkAttemptCompletionPolicy/minFailedTaskCount",
    "value": $(MinFailedTaskCount)
  }
]
```

Type: application/json-patch+json

**Description**

Update the TaskNumber (and the FrameworkAttemptCompletionPolicy) of the specified TaskRole in the specified Framework.

See more in [Framework ScaleUp/ScaleDown](#FrameworkRescale).

**Response**

| Code | Body | Description |
|:---- |:---- |:---- |
| OK(200) | [Framework](../pkg/apis/frameworkcontroller/v1/types.go) | Return current Framework. |
| NotFound(404) | [Status](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#status-v1-meta) | The specified Framework is not found. |
| UnprocessableEntity(422) | [Status](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#status-v1-meta) | The specified $(TaskRoleName) does not exist or does not match the specified $(TaskRoleIndex). |

#### <a name="DELETE_Framework">DELETE Framework</a>
**Request**

    DELETE /apis/frameworkcontroller.microsoft.com/v1/namespaces/{FrameworkNamespace}/frameworks/{FrameworkName}

Body:

application/json
```json
{
  "propagationPolicy": "Foreground"
}
```
application/yaml
```yaml
propagationPolicy: Foreground
```

Type: application/json or application/yaml

**Description**

Delete the specified Framework.

Notes:
* If you need to achieve all the [Framework ConsistencyGuarantees](#ConsistencyGuarantees) or achieve higher [Framework Availability](#FrameworkAvailability) by leveraging the [PodGracefulDeletionTimeoutSec](../pkg/apis/frameworkcontroller/v1/types.go), you should always use and only use the [Foreground Deletion](https://kubernetes.io/docs/concepts/workloads/controllers/garbage-collection/#foreground-cascading-deletion) in the provided body.
* However, `kubectl delete` does not support to specify the Foreground Deletion at least for [Kubernetes v1.14.2](https://github.com/kubernetes/kubernetes/issues/66110#issuecomment-413761559), so you may have to use other [Supported Client](#SupportedClient).

**Response**

| Code | Body | Description |
|:---- |:---- |:---- |
| OK(200) | [Framework](../pkg/apis/frameworkcontroller/v1/types.go) | The specified Framework is deleting.<br>Return current Framework. |
| OK(200) | [Status](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#status-v1-meta) | The specified Framework is deleted. |
| NotFound(404) | [Status](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#status-v1-meta) | The specified Framework is not found. |

#### <a name="GET_Framework">GET Framework</a>
**Request**

    GET /apis/frameworkcontroller.microsoft.com/v1/namespaces/{FrameworkNamespace}/frameworks/{FrameworkName}

**Description**

Get the specified Framework.

**Response**

| Code | Body | Description |
|:---- |:---- |:---- |
| OK(200) | [Framework](../pkg/apis/frameworkcontroller/v1/types.go) | Return current Framework. |
| NotFound(404) | [Status](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#status-v1-meta) | The specified Framework is not found. |

#### <a name="LIST_Frameworks">LIST Frameworks</a>
**Request**

    GET /apis/frameworkcontroller.microsoft.com/v1/namespaces/{FrameworkNamespace}/frameworks
    GET /apis/frameworkcontroller.microsoft.com/v1/frameworks

QueryParameters: Same as [StatefulSet QueryParameters](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#list-statefulset-v1-apps)

**Description**

Get all Frameworks (in the specified FrameworkNamespace).

**Response**

| Code | Body | Description |
|:---- |:---- |:---- |
| OK(200) | [FrameworkList](../pkg/apis/frameworkcontroller/v1/types.go) | Return all Frameworks (in the specified FrameworkNamespace). |

#### <a name="WATCH_Framework">WATCH Framework</a>
**Request**

    GET /apis/frameworkcontroller.microsoft.com/v1/watch/namespaces/{FrameworkNamespace}/frameworks/{FrameworkName}

QueryParameters: Same as [StatefulSet QueryParameters](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#watch-statefulset-v1-apps)

**Description**

Watch the change events of the specified Framework.

**Response**

| Code | Body | Description |
|:---- |:---- |:---- |
| OK(200) | [WatchEvent](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#watchevent-v1-meta) | Streaming the change events of the specified Framework. |
| NotFound(404) | [Status](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#status-v1-meta) | The specified Framework is not found. |

#### <a name="WATCH_LIST_Frameworks">WATCH_LIST Frameworks</a>
**Request**

    GET /apis/frameworkcontroller.microsoft.com/v1/watch/namespaces/{FrameworkNamespace}/frameworks
    GET /apis/frameworkcontroller.microsoft.com/v1/watch/frameworks

QueryParameters: Same as [StatefulSet QueryParameters](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#watch-list-statefulset-v1-apps)

**Description**

Watch the change events of all Frameworks (in the specified FrameworkNamespace).

**Response**

| Code | Body | Description |
|:---- |:---- |:---- |
| OK(200) | [WatchEvent](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#watchevent-v1-meta) | Streaming the change events of all Frameworks (in the specified FrameworkNamespace). |

## <a name="ContainerEnvironmentVariable">Container EnvironmentVariable</a>
[Container EnvironmentVariable](../pkg/apis/frameworkcontroller/v1/constants.go)

## <a name="PodFailureClassification">Pod Failure Classification</a>
You can specify how to classify and summarize Pod failures by the [PodFailureSpec](../pkg/apis/frameworkcontroller/v1/config.go).

You can also directly leverage the [Default PodFailureSpec](../example/config/default/frameworkcontroller.yaml).

## <a name="PredefinedCompletionCode">Predefined CompletionCode</a>
You can leverage the [Predefined CompletionCode](../pkg/apis/frameworkcontroller/v1/completion.go) to instruct your [RetryPolicy](#RetryPolicy) and identify a certain predefined CompletionCode, regardless of different [PodFailureSpec](../pkg/apis/frameworkcontroller/v1/config.go) may be configured in different clusters.

## <a name="CompletionStatus">CompletionStatus</a>
[CompletionStatus](../pkg/apis/frameworkcontroller/v1/types.go): It is generated from [Predefined CompletionCode](#PredefinedCompletionCode) or [PodPattern matching](#PodFailureClassification). For a Pod, if no PodPattern is matched and failed Container exists, the CompletionCode is the same as the last failed Container ExitCode.

[TaskAttemptCompletionStatus](../pkg/apis/frameworkcontroller/v1/types.go): Besides the [CompletionStatus](../pkg/apis/frameworkcontroller/v1/types.go), it also provides more detailed and structured diagnostic information about the completion of a TaskAttempt.

[FrameworkAttemptCompletionStatus](../pkg/apis/frameworkcontroller/v1/types.go): Besides the [CompletionStatus](../pkg/apis/frameworkcontroller/v1/types.go), it also provides more detailed and structured diagnostic information about the completion of a FrameworkAttempt.

## <a name="RetryPolicy">RetryPolicy</a>
### <a name="RetryPolicy_Spec">Spec</a>
[RetryPolicySpec](../pkg/apis/frameworkcontroller/v1/types.go)

### <a name="RetryPolicy_Usage">Usage</a>
[RetryPolicySpec](../pkg/apis/frameworkcontroller/v1/types.go)

### <a name="RetryPolicy_Example">Example</a>
Notes:
1. *Italic Conditions* still need to be specified explicitly, as we have not supported the Framework Spec Defaulting yet.
2. For the definition of each [CompletionType](../pkg/apis/frameworkcontroller/v1/types.go), such as Transient Failed, see [CompletionStatus](#CompletionStatus).

<table>
  <tbody>
    <tr>
      <th>FrameworkType</th>
      <th>Framework RetryPolicy</th>
      <th>TaskRole</th>
      <th>Task RetryPolicy</th>
      <th>Description</th>
    </tr>
    <tr>
      <td rowspan="2"><b>DEFAULT</td>
      <td rowspan="2"><i>FancyRetryPolicy = false<br>MaxRetryCount = 0</i></td>
      <td>TaskRole-A</td>
      <td><i>FancyRetryPolicy = false<br>MaxRetryCount = 0</i></td>
      <td rowspan="2">The default RetryPolicy:<br>Never Retry for any Failed or Succeeded.</td>
    </tr>
    <tr>
      <td>TaskRole-B</td>
      <td><i>FancyRetryPolicy = false<br>MaxRetryCount = 0</i></td>
    </tr>
    <tr>
      <td rowspan="1"><b>Service</td>
      <td rowspan="1"><i>FancyRetryPolicy = false</i><br>MaxRetryCount = -2</td>
      <td>TaskRole-A</td>
      <td><i>FancyRetryPolicy = false</i><br>MaxRetryCount = -2</td>
      <td rowspan="1">Always Retry for any Failed or Succeeded.</td>
    </tr>
    <tr>
      <td rowspan="1"><b>Blind Batch</td>
      <td rowspan="1"><i>FancyRetryPolicy = false</i><br>MaxRetryCount = -1</td>
      <td>TaskRole-A</td>
      <td><i>FancyRetryPolicy = false</i><br>MaxRetryCount = -1</td>
      <td rowspan="1">Always Retry for any Failed.<br>Never Retry for Succeeded.</td>
    </tr>
    <tr>
      <td rowspan="1"><b>Batch with Task Fault Tolerance</td>
      <td rowspan="1">FancyRetryPolicy = true<br>MaxRetryCount = 3</td>
      <td>TaskRole-A</td>
      <td>FancyRetryPolicy = true<br>MaxRetryCount = 3</td>
      <td rowspan="1">Always Retry for Transient Failed.<br>Never Retry for Permanent Failed or Succeeded.<br>Retry up to 3 times for Unknown Failed.</td>
    </tr>
    <tr>
      <td rowspan="1"><b>Batch without Task Fault Tolerance</td>
      <td rowspan="1">FancyRetryPolicy = true<br>MaxRetryCount = 3</td>
      <td>TaskRole-A</td>
      <td><i>FancyRetryPolicy = false<br>MaxRetryCount = 0</i></td>
      <td rowspan="1">For Framework RetryPolicy, same as "Batch with Task Fault Tolerance".<br>For Task RetryPolicy, because the Task cannot tolerate any failed TaskAttempt, such as it cannot recover from previous failed TaskAttempt, so Never Retry Task for any Failed or Succeeded.</td>
    </tr>
    <tr>
      <td rowspan="1"><b>Debug Mode</td>
      <td rowspan="1">FancyRetryPolicy = true<br><i>MaxRetryCount = 0</i></td>
      <td>TaskRole-A</td>
      <td>FancyRetryPolicy = true<br><i>MaxRetryCount = 0</i></td>
      <td rowspan="1">Always Retry for Transient Failed.<br>Never Retry for Permanent Failed or Unknown Failed or Succeeded.<br>This can help to capture the unexpected exit of user application itself.</td>
    </tr>
  </tbody>
</table>

## <a name="FrameworkAttemptCompletionPolicy">FrameworkAttemptCompletionPolicy</a>
### <a name="FrameworkAttemptCompletionPolicy_Spec">Spec</a>
[CompletionPolicySpec](../pkg/apis/frameworkcontroller/v1/types.go)

### <a name="FrameworkAttemptCompletionPolicy_Usage">Usage</a>
[CompletionPolicySpec](../pkg/apis/frameworkcontroller/v1/types.go)

### <a name="FrameworkAttemptCompletionPolicy_Example">Example</a>
Notes:
1. *Italic Conditions* still need to be specified explicitly, as we have not supported the Framework Spec Defaulting yet.

<table>
  <tbody>
    <tr>
      <th>FrameworkType</th>
      <th>TaskRole</th>
      <th>FrameworkAttemptCompletionPolicy</th>
      <th>Description</th>
    </tr>
    <tr>
      <td rowspan="2"><b>DEFAULT</td>
      <td>TaskRole-A</td>
      <td><i>MinFailedTaskCount = 1<br>MinSucceededTaskCount = -1</i></td>
      <td rowspan="2">The default FrameworkAttemptCompletionPolicy:<br>Fail the FrameworkAttempt immediately if any Task failed.<br>Succeed the FrameworkAttempt until all Tasks succeeded.</td>
    </tr>
    <tr>
      <td>TaskRole-B</td>
      <td><i>MinFailedTaskCount = 1<br>MinSucceededTaskCount = -1</i></td>
    </tr>
    <tr>
      <td rowspan="1"><b>Service</td>
      <td>TaskRole-A</td>
      <td><i>MinFailedTaskCount = 1<br>MinSucceededTaskCount = -1</i></td>
      <td rowspan="1">Actually, any FrameworkAttemptCompletionPolicy is fine, since Service's Task will never complete, i.e. its Task's MaxRetryCount is -2, see <a href="#RetryPolicy_Example">RetryPolicy Example</a>.</td>
    </tr>
    <tr>
      <td rowspan="2"><b>MapReduce</td>
      <td>Map</td>
      <td>MinFailedTaskCount = {Map.TaskNumber} * {mapreduce.map.failures.maxpercent} + 1<br><i>MinSucceededTaskCount = -1</i></td>
      <td rowspan="2">A few failed Tasks is acceptable, but always want to wait all Tasks to succeed:<br>Fail the FrameworkAttempt immediately if the failed Tasks exceeded the limit.<br>Succeed the FrameworkAttempt until all Tasks completed and the failed Tasks is within the limit.</td>
    </tr>
    <tr>
      <td>Reduce</td>
      <td>MinFailedTaskCount = {Reduce.TaskNumber} * {mapreduce.reduce.failures.maxpercent} + 1<br><i>MinSucceededTaskCount = -1</i></td>
    </tr>
    <tr>
      <td rowspan="2"><b>TensorFlow</td>
      <td>ParameterServer</td>
      <td><i>MinFailedTaskCount = 1<br>MinSucceededTaskCount = -1</i></td>
      <td rowspan="2">Succeed a certain TaskRole is enough, and do not want to wait all Tasks to succeed:<br>Fail the FrameworkAttempt immediately if any Task failed.<br>Succeed the FrameworkAttempt immediately if Worker's all Tasks succeeded.</td>
    </tr>
    <tr>
      <td>Worker</td>
      <td><i>MinFailedTaskCount = 1</i><br>MinSucceededTaskCount = {Worker.TaskNumber}</td>
    </tr>
    <tr>
      <td rowspan="3"><b>Arbitrator Dominated</td>
      <td>Arbitrator</td>
      <td><i>MinFailedTaskCount = 1</i><br>MinSucceededTaskCount = 1</td>
      <td rowspan="3">The FrameworkAttemptCompletionPolicy is fully delegated to the single instance arbitrator of the user application:<br>Fail the FrameworkAttempt immediately if the arbitrator failed.<br>Succeed the FrameworkAttempt immediately if the arbitrator succeeded.</td>
    </tr>
    <tr>
      <td>TaskRole-A</td>
      <td>MinFailedTaskCount = -1<br><i>MinSucceededTaskCount = -1</i></td>
    </tr>
    <tr>
      <td>TaskRole-B</td>
      <td>MinFailedTaskCount = -1<br><i>MinSucceededTaskCount = -1</i></td>
    </tr>
    <tr>
      <td rowspan="1"><b>First Completed Task Dominated</td>
      <td>TaskRole-A</td>
      <td><i>MinFailedTaskCount = 1</i><br>MinSucceededTaskCount = 1</td>
      <td rowspan="1">The FrameworkAttemptCompletionPolicy is fully delegated to the first completed Task of the user application:<br>Fail the FrameworkAttempt immediately if any Task failed.<br>Succeed the FrameworkAttempt immediately if any Task succeeded.</td>
    </tr>
  </tbody>
</table>

## <a name="FrameworkRescale">Framework ScaleUp/ScaleDown</a>
Framework ScaleUp/ScaleDown (Rescale) refers to take any below action for an existing Framework on the fly:
1. Add/Delete TaskRole without touching other TaskRoles.
2. Add/Delete Task without touching other Tasks.

### <a name="FrameworkRescaleApplicationAssumption">Application Assumption</a>
Before you start to Rescale Framework, make sure your application executed by the Framework can tolerate Rescale, such as:
1. Your application should be able to **rebalance** its workload after Rescale:
   1. For Service application, may need to rebalance its serving traffic by a load balancer, and/or re-replicate its state.
   2. For Batch application, may need to rebalance its work items by a work queue, and/or adjust its Tasks membership, like [PyTorch Elastic Training](https://pytorch.org/elastic).
2. For **Batch** application, it would better **not rerun** after ScaleUp:
   1. Rerun may happen if the application already completed, but the completion event have not yet been observed by FrameworkController. So, during this period, if you ScaleUp the Framework, the application may rerun.
   2. To mitigate it, the ScaleUp Task can immediately complete itself by leveraging empty work queue or existing checkpoint from previous run.
3. For **Batch** application, it would better **not too early succeeded** after ScaleDown:
   1. Too early succeeded may happen if all Tasks succeeded except one Task still running, but you ScaleDown the running Task.
   2. To resolve it, make sure it is safe to ScaleDown the running Task, such as leverage `First Completed Task Dominated` or `Arbitrator Dominated` FrameworkType in [FrameworkAttemptCompletionPolicy](#FrameworkAttemptCompletionPolicy). For the `First Completed Task Dominated` FrameworkType, an exit barrier may be needed to ensure any Task succeeded means the whole application already succeeded, like [PyTorch Elastic Training](https://pytorch.org/elastic). For the `Arbitrator Dominated` FrameworkType, an arbitrator TaskRole is needed and do not ScaleDown the arbitrator TaskRole.

### <a name="FrameworkRescaleAPI">API</a>
- [Add TaskRole](#Add_TaskRole)
- [Delete TaskRole](#Delete_TaskRole)
- [Add/Delete Task](#Add_Delete_Task)

### <a name="FrameworkRescaleExample">Example</a>
#### <a name="FrameworkRescaleBasicExample">Basic Example</a>
This example will demonstrate the basic usage of Framework Rescale, as well as its [Strong Safety Guarantee](#FrameworkRescaleGuarantee).
1. [Create Framework](#CREATE_Framework) as below, and wait until all Tasks are AttemptRunning:
```yaml
apiVersion: frameworkcontroller.microsoft.com/v1
kind: Framework
metadata:
  name: rescalebasic
spec:
  executionType: Start
  retryPolicy:
    fancyRetryPolicy: false
    maxRetryCount: 0
  taskRoles:
    - name: a
      taskNumber: 4
      frameworkAttemptCompletionPolicy:
        minFailedTaskCount: 4
        minSucceededTaskCount: 1
      task:
        retryPolicy:
          fancyRetryPolicy: false
          maxRetryCount: 0
        pod:
          spec:
            restartPolicy: Never
            containers:
              - name: ubuntu
                image: ubuntu:trusty
                command: ["sh", "-c", "printenv && sleep infinity"]
```
2. Delete Pod `rescalebasic-a-2`, `rescalebasic-a-3`, and wait until their Tasks are Completed (Failed).
3. [ScaleDown Framework](#Add_Delete_Task): Decrease the taskNumber and minFailedTaskCount from 4 to 2 by below patch:
```json
[
  {
    "op": "test",
    "path": "/spec/taskRoles/0/name",
    "value": "a"
  },
  {
    "op": "replace",
    "path": "/spec/taskRoles/0/taskNumber",
    "value": 2
  },
  {
    "op": "replace",
    "path": "/spec/taskRoles/0/frameworkAttemptCompletionPolicy/minFailedTaskCount",
    "value": 2
  }
]
```
4. The Completed Tasks `rescalebasic-a-2`, `rescalebasic-a-3` are immediately deleted, and the Framework stays AttemptRunning.
   - This demonstrates [SafetyGuarantee1](#FrameworkRescaleGuarantee), as otherwise, the old Failed Tasks `rescalebasic-a-2`, `rescalebasic-a-3` may be wrongly considered in the new FrameworkAttemptCompletionPolicy.minFailedTaskCount (2) and triggers the completion.
5. [ScaleUp Framework](#Add_Delete_Task): Increase the taskNumber and minFailedTaskCount from 2 to 4 by below patch, and wait until all Tasks are AttemptRunning:
```json
[
  {
    "op": "test",
    "path": "/spec/taskRoles/0/name",
    "value": "a"
  },
  {
    "op": "replace",
    "path": "/spec/taskRoles/0/taskNumber",
    "value": 4
  },
  {
    "op": "replace",
    "path": "/spec/taskRoles/0/frameworkAttemptCompletionPolicy/minFailedTaskCount",
    "value": 4
  }
]
```
6. Delete Pod `rescalebasic-a-2`, and wait until its Task is Completed (Failed).
7. Redo Step 3, and wait until the `rescalebasic-a-2`, `rescalebasic-a-3` Tasks are DeletionPending, but before `rescalebasic-a-3` is deleted, then immediately [ScaleUp Framework](#Add_Delete_Task): Increase the taskNumber and minFailedTaskCount from 2 to 3 by below patch:
```json
[
  {
    "op": "test",
    "path": "/spec/taskRoles/0/name",
    "value": "a"
  },
  {
    "op": "replace",
    "path": "/spec/taskRoles/0/taskNumber",
    "value": 3
  },
  {
    "op": "replace",
    "path": "/spec/taskRoles/0/frameworkAttemptCompletionPolicy/minFailedTaskCount",
    "value": 3
  }
]
```
8. The Completed Task `rescalebasic-a-2` is immediately replaced by a new Task instance, the AttemptRunning Task `rescalebasic-a-3` is eventually deleted, and the Framework stays AttemptRunning.
   - This demonstrates [SafetyGuarantee2](#FrameworkRescaleGuarantee), as otherwise, the previous ScaleDown Task `rescalebasic-a-2` may be wrongly reused in later ScaleUp.
9. [ScaleUp Framework](#Add_Delete_Task): Increase the taskNumber and minFailedTaskCount from 3 to 4 by below patch, and wait until all Tasks are AttemptRunning:
```json
[
  {
    "op": "test",
    "path": "/spec/taskRoles/0/name",
    "value": "a"
  },
  {
    "op": "replace",
    "path": "/spec/taskRoles/0/taskNumber",
    "value": 4
  },
  {
    "op": "replace",
    "path": "/spec/taskRoles/0/frameworkAttemptCompletionPolicy/minFailedTaskCount",
    "value": 4
  }
]
```
10. Delete Pod `rescalebasic-a-2`, and wait until its Task is Completed (Failed).
11. Redo Step 3, and wait until the `rescalebasic-a-2`, `rescalebasic-a-3` Tasks are DeletionPending, but before `rescalebasic-a-3` is deleted, then immediately [ScaleUp Framework](#Add_Delete_Task): Increase the taskNumber and minFailedTaskCount from 2 to 5 by below patch:
```json
[
  {
    "op": "test",
    "path": "/spec/taskRoles/0/name",
    "value": "a"
  },
  {
    "op": "replace",
    "path": "/spec/taskRoles/0/taskNumber",
    "value": 5
  },
  {
    "op": "replace",
    "path": "/spec/taskRoles/0/frameworkAttemptCompletionPolicy/minFailedTaskCount",
    "value": 5
  }
]
```
12. The Completed Task `rescalebasic-a-2` is immediately replaced by a new Task instance, the AttemptRunning Task `rescalebasic-a-3` is eventually replaced by a new Task instance, the new Task `rescalebasic-a-4` is immediately added, and the Framework stays AttemptRunning.
    - This demonstrates [SafetyGuarantee2](#FrameworkRescaleGuarantee), as otherwise, the previous ScaleDown Task `rescalebasic-a-2`, `rescalebasic-a-3` may be wrongly reused in later ScaleUp.

#### <a name="FrameworkRescalePETExample">PyTorch Elastic Training Example</a>

### <a name="FrameworkRescalePipeline">Pipeline</a>
**ScaleUp Pipeline**:
1. As soon as FrameworkController observes not Completing/Completed Framework ScaleUp request, it will immediately mark the ScaleUp Task as [AttemptCreationPending](../pkg/apis/frameworkcontroller/v1/types.go) then persist (expose) the Task, before take any other action for the Framework, such as start to create its TaskAttempt or evaluate the Task's impact to the Framework, such as [FrameworkAttemptCompletionPolicy](#FrameworkAttemptCompletionPolicy).
2. As soon as the AttemptCreationPending Task is persisted (exposed), the Task will impact its Framework in future, such as:
   1. The Task will be considered in FrameworkAttemptCompletionPolicy.
   2. The Task will be deleted in later ScaleDown.
3. Only until then, FrameworkController will start to create the AttemptCreationPending TaskAttempt.

**ScaleDown Pipeline**:
1. As soon as FrameworkController observes not Completing/Completed Framework ScaleDown request, it will immediately mark the ScaleDown Task as [DeletionPending](../pkg/apis/frameworkcontroller/v1/types.go) then persist (expose) the Task, before take any other action for the Framework, such as start to delete the Task or evaluate the Task's impact to the Framework, such as [FrameworkAttemptCompletionPolicy](#FrameworkAttemptCompletionPolicy).
2. As soon as the DeletionPending Task is persisted (exposed), the Task will never impact its Framework in future anymore except for the Task's graceful deletion period itself, such as:
   1. The Task will never be considered in FrameworkAttemptCompletionPolicy.
   2. The Task will never be reused in later ScaleUp.
3. Only until then, FrameworkController will start to delete the DeletionPending Task.
4. After Framework Completing/Completed, the Framework may still contain DeletionPending Tasks but these Tasks must be Completed.

**Notes**:
1. User also needs to explicitly ignore the DeletionPending Task if he does not care about the Task's graceful deletion period, like [FrameworkBarrier](../pkg/barrier/barrier.go).

### <a name="FrameworkRescaleGuarantee">Strong Safety Guarantee</a>
Besides these general [Framework ConsistencyGuarantees](#ConsistencyGuarantees), FrameworkController also provides below Strong Safety Guarantees for Framework Rescale:
- **SafetyGuarantee1**:
  - User can always safely Rescale Framework and update its other Spec within a single Framework update, as if the Rescale is already done, such as [also update the FrameworkAttemptCompletionPolicy according to the new scale](#Add_Delete_Task), as:
     - The ScaleUp/ScaleDown Task will be immediately ignored/considered for the updated Framework.
- **SafetyGuarantee2**:
  - User can always safely Rescale before any previous Rescale totally finished (including the ScaleDown Task final deletion), as:
    - For ScaleUp immediately followed by a ScaleDown:
      - If the user observed a new non-[DeletionPending](../pkg/apis/frameworkcontroller/v1/types.go) Task (caused by ScaleUp), later ScaleDown will delete it (i.e. ScaleUp committed), otherwise, later ScaleDown may not delete it but previous ScaleUp must not have impacted it, such as start to create its TaskAttempt (i.e. ScaleUp rollbacked).
    - For ScaleDown immediately followed by a ScaleUp:
      - If the user observed a new [DeletionPending](../pkg/apis/frameworkcontroller/v1/types.go) Task (caused by ScaleDown), later ScaleUp will not reuse it (i.e. ScaleDown committed), otherwise, later ScaleUp may reuse it but previous ScaleDown must not have impacted it, such as start to delete it (i.e. ScaleDown rollbacked).

## <a name="LargeScaleFramework">Large Scale Framework</a>
To safely run large scale Framework, i.e. the total task number in a single Framework is greater than 300, you just need to enable the [LargeFrameworkCompression](../pkg/apis/frameworkcontroller/v1/config.go). However, you may also need to decompress the Framework by yourself.

## <a name="FrameworkPodHistory">Framework and Pod History</a>
By leveraging the [LogObjectSnapshot](../pkg/apis/frameworkcontroller/v1/config.go), external systems, such as [Fluentd](https://www.fluentd.org) and [ElasticSearch](https://www.elastic.co/products/elasticsearch), can collect and process Framework and Pod history snapshots even if it was retried or deleted, such as persistence, metrics conversion, visualization, alerting, acting, analysis, etc.

## <a name="FrameworkTaskStateMachine">Framework and Task State Machine</a>
### <a name="FrameworkStateMachine">Framework State Machine</a>
[FrameworkState](../pkg/apis/frameworkcontroller/v1/types.go)

### <a name="TaskStateMachine">Task State Machine</a>
[TaskState](../pkg/apis/frameworkcontroller/v1/types.go)

## <a name="FrameworkConsistencyAvailability">Framework Consistency vs Availability</a>
### <a name="FrameworkConsistency">Framework Consistency</a>
#### <a name="ConsistencyGuarantees">ConsistencyGuarantees</a>
For a specific Task identified by {FrameworkName}-{TaskRoleName}-{TaskIndex}:

- **ConsistencyGuarantee1**:

  At most one instance of the Task is running at any point in time.

- **ConsistencyGuarantee2**:

  No instance of the Task is running if it is deleted (or does not exist), TaskAttemptCompleted, TaskCompleted or the whole Framework is deleted (or does not exist).

For a specific Framework identified by {FrameworkName}:

- **ConsistencyGuarantee3**:

  At most one instance of the Framework is running at any point in time.

- **ConsistencyGuarantee4**:

  No instance of the Framework is running if it is FrameworkAttemptCompleted, FrameworkCompleted or the whole Framework is deleted (or does not exist).

#### <a name="ConsistencyGuaranteesHowTo">How to achieve ConsistencyGuarantees</a>

The default behavior is to achieve all the [ConsistencyGuarantees](#ConsistencyGuarantees), if you do not explicitly violate below guidelines:

1. Achieve **ConsistencyGuarantee1**:

    Do not [force delete the managed Pod](https://kubernetes.io/docs/concepts/workloads/pods/pod/#force-deletion-of-pods):

   1. Do not set [PodGracefulDeletionTimeoutSec](../pkg/apis/frameworkcontroller/v1/types.go) to be not nil.

      For example, the default PodGracefulDeletionTimeoutSec is acceptable.

   2. Do not delete the managed Pod with [0 GracePeriodSeconds](https://kubernetes.io/docs/concepts/workloads/pods/pod/#force-deletion-of-pods).

      For example, the default Pod deletion is acceptable.

   3. Do not delete the Node which runs the managed Pod.

      For example, [drain the Node](https://kubernetes.io/docs/tasks/administer-cluster/safely-drain-node) before delete it is acceptable.

   *The Task instance can be universally located by its [TaskAttemptInstanceUID](../pkg/apis/frameworkcontroller/v1/types.go) or [PodUID](../pkg/apis/frameworkcontroller/v1/types.go).*

   *To avoid the Pod is stuck in deleting forever, such as if its Node is down forever, leverage the same approach as [Delete StatefulSet Pod only after the Pod termination has been confirmed](https://kubernetes.io/docs/tasks/run-application/force-delete-stateful-set-pod/#delete-pods) manually or by your [Cloud Controller Manager](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/#running-cloud-controller-manager).*

2. Achieve **ConsistencyGuarantee2**, **ConsistencyGuarantee3** and **ConsistencyGuarantee4**:
   1. Achieve **ConsistencyGuarantee1**.

   2. Must delete the managed ConfigMap with [Foreground PropagationPolicy](https://kubernetes.io/docs/concepts/workloads/controllers/garbage-collection/#foreground-cascading-deletion).

      For example, the default ConfigMap deletion is acceptable.

   3. Must delete the Framework with [Foreground PropagationPolicy](https://kubernetes.io/docs/concepts/workloads/controllers/garbage-collection/#foreground-cascading-deletion).

      For example, the default Framework deletion may not be acceptable, since the default PropagationPolicy for Framework object may be Background.

   4. Do not change the [OwnerReferences](https://kubernetes.io/docs/concepts/workloads/controllers/garbage-collection/#owners-and-dependents) of the managed ConfigMap and Pods.

   *The Framework instance can be universally located by its [FrameworkAttemptInstanceUID](../pkg/apis/frameworkcontroller/v1/types.go) or [ConfigMapUID](../pkg/apis/frameworkcontroller/v1/types.go).*

### <a name="FrameworkAvailability">Framework Availability</a>
According to the [CAP theorem](https://en.wikipedia.org/wiki/CAP_theorem), in the presence of a network partition, you cannot achieve both consistency and availability at the same time in any distributed system. So you have to make a trade-off between the [Framework Consistency](#FrameworkConsistency) and the [Framework Availability](#FrameworkAvailability).

You can tune the trade-off, such as to achieve higher [Framework Availability](#FrameworkAvailability) by sacrificing the [Framework Consistency](#FrameworkConsistency):
1. Decrease [Pod TolerationSeconds for TaintBasedEvictions](https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/#taint-based-evictions)
2. Increase [Node Eviction Rate](https://kubernetes.io/docs/concepts/architecture/nodes/#node-controller)
3. Set a small [PodGracefulDeletionTimeoutSec](../pkg/apis/frameworkcontroller/v1/types.go)
4. Violate other guidelines mentioned in [How to achieve ConsistencyGuarantees](#ConsistencyGuaranteesHowTo), such as manually force delete a problematic Pod.

See more in:
1. [PodGracefulDeletionTimeoutSec](../pkg/apis/frameworkcontroller/v1/types.go)
2. [Pod Safety and Consistency Guarantees](https://github.com/kubernetes/community/blob/ee8998b156031f6b363daade51ca2d12521f4ac0/contributors/design-proposals/storage/pod-safety.md)

## <a name="ControllerExtension">Controller Extension</a>
### <a name="FrameworkBarrier">FrameworkBarrier</a>
1. [Usage](../pkg/barrier/barrier.go)
2. Example: [FrameworkBarrier Example](../example/framework/extension/frameworkbarrier.yaml), [TensorFlow Example](../example/framework/scenario/tensorflow), [etc](../example/framework/scenario).

### <a name="HiveDScheduler">HiveDScheduler</a>
1. [Usage](https://github.com/microsoft/hivedscheduler)
2. Example: [TensorFlow Example](../example/framework/scenario/tensorflow/gpu/tensorflowdistributedtrainingwithhivedscheduledgpu.yaml), [etc](https://github.com/microsoft/hivedscheduler/blob/master/example/request/design/request.yaml).

## <a name="BestPractice">Best Practice</a>
[Best Practice](../pkg/apis/frameworkcontroller/v1/types.go)
