# <a name="UserManual">User Manual</a>

## <a name="Index">Index</a>
   - [Framework Interop](#FrameworkInterop)
   - [Container EnvironmentVariable](#ContainerEnvironmentVariable)
   - [CompletionCode Convention](#CompletionCodeConvention)
   - [RetryPolicy](#RetryPolicy)
   - [FrameworkAttemptCompletionPolicy](#FrameworkAttemptCompletionPolicy)
   - [Controller Extension](#ControllerExtension)
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
| Framework | [CREATE](#CREATE_Framework) [DELETE](#DELETE_Framework) [GET](#GET_Framework) [LIST](#LIST_Frameworks) [WATCH](#WATCH_Framework) [WATCH_LIST](#WATCH_LIST_Frameworks) |
| [ConfigMap](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#configmap-v1-core) | All operations except for [CREATE](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#create-193) [PUT](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#replace-195) [PATCH](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#patch-194) |
| [Pod](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#pod-v1-core) | All operations except for [CREATE](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#create-55) [PUT](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#replace-57) [PATCH](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#patch-56) |

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
| Conflict(409) | [Status](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#status-v1-meta) | The specified Framework already exists. |

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
* If you need to ensure at most one instance of a specific Framework (identified by the FrameworkName) is running at any point in time, you should always use and only use the [Foreground Deletion](https://kubernetes.io/docs/concepts/workloads/controllers/garbage-collection/#foreground-cascading-deletion) in the provided body, see [Framework Notes](../pkg/apis/frameworkcontroller/v1/types.go). However, `kubectl delete` does not support to specify the Foreground Deletion at least for [Kubernetes v1.10](https://github.com/kubernetes/kubernetes/issues/66110#issuecomment-413761559), so you may have to use other [Supported Client](#SupportedClient).

**Response**

| Code | Body | Description |
|:---- |:---- |:---- |
| OK(200) | [Framework](../pkg/apis/frameworkcontroller/v1/types.go) | The specified Framework is deleting.<br>Return current Framework. |
| OK(200) | [Status](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#status-v1-meta) | The specified Framework is deleted. |
| NotFound(200) | [Status](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#status-v1-meta) | The specified Framework is not found. |

#### <a name="GET_Framework">GET Framework</a>
**Request**

    GET /apis/frameworkcontroller.microsoft.com/v1/namespaces/{FrameworkNamespace}/frameworks/{FrameworkName}

**Description**

Get the specified Framework.

**Response**

| Code | Body | Description |
|:---- |:---- |:---- |
| OK(200) | [Framework](../pkg/apis/frameworkcontroller/v1/types.go) | Return current Framework. |
| NotFound(200) | [Status](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#status-v1-meta) | The specified Framework is not found. |

#### <a name="LIST_Frameworks">LIST Frameworks</a>
**Request**

    GET /apis/frameworkcontroller.microsoft.com/v1/namespaces/{FrameworkNamespace}/frameworks
    GET /apis/frameworkcontroller.microsoft.com/v1/frameworks

QueryParameters: Same as [StatefulSet QueryParameters](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#list-122)

**Description**

Get all Frameworks (in the specified FrameworkNamespace).

**Response**

| Code | Body | Description |
|:---- |:---- |:---- |
| OK(200) | [FrameworkList](../pkg/apis/frameworkcontroller/v1/types.go) | Return all Frameworks (in the specified FrameworkNamespace). |

#### <a name="WATCH_Framework">WATCH Framework</a>
**Request**

    GET /apis/frameworkcontroller.microsoft.com/v1/watch/namespaces/{FrameworkNamespace}/frameworks/{FrameworkName}

QueryParameters: Same as [StatefulSet QueryParameters](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#watch-124)

**Description**

Watch the change events of the specified Framework.

**Response**

| Code | Body | Description |
|:---- |:---- |:---- |
| OK(200) | [WatchEvent](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#watchevent-v1-meta) | Streaming the change events of the specified Framework. |
| NotFound(200) | [Status](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#status-v1-meta) | The specified Framework is not found. |

#### <a name="WATCH_LIST_Frameworks">WATCH_LIST Frameworks</a>
**Request**

    GET /apis/frameworkcontroller.microsoft.com/v1/watch/namespaces/{FrameworkNamespace}/frameworks
    GET /apis/frameworkcontroller.microsoft.com/v1/watch/frameworks

QueryParameters: Same as [StatefulSet QueryParameters](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#watch-list-125)

**Description**

Watch the change events of all Frameworks (in the specified FrameworkNamespace).

**Response**

| Code | Body | Description |
|:---- |:---- |:---- |
| OK(200) | [WatchEvent](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#watchevent-v1-meta) | Streaming the change events of all Frameworks (in the specified FrameworkNamespace). |

## <a name="ContainerEnvironmentVariable">Container EnvironmentVariable</a>
[Container EnvironmentVariable](../pkg/apis/frameworkcontroller/v1/constants.go)

## <a name="CompletionCodeConvention">CompletionCode Convention</a>
[CompletionCode Convention](../pkg/apis/frameworkcontroller/v1/constants.go)

## <a name="RetryPolicy">RetryPolicy</a>
### <a name="RetryPolicy_Spec">Spec</a>
[RetryPolicySpec](../pkg/apis/frameworkcontroller/v1/types.go)

### <a name="RetryPolicy_Usage">Usage</a>
[RetryPolicySpec](../pkg/apis/frameworkcontroller/v1/types.go)

### <a name="RetryPolicy_Example">Example</a>
Notes:
1. *Italic Conditions* can be inherited from the **DEFAULT** RetryPolicy, so no need to specify them explicitly.

   *You still need to specify them explicitly, as we have not supported the Framework Spec Defaulting yet.*

2. For the definition of each CompletionType, such as Transient Failed, see [CompletionCode Convention](#CompletionCodeConvention).

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
1. *Italic Conditions* can be inherited from the **DEFAULT** FrameworkAttemptCompletionPolicy, so no need to specify them explicitly.

   *You still need to specify them explicitly, as we have not supported the Framework Spec Defaulting yet.*

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

## <a name="ControllerExtension">Controller Extension</a>
### <a name="FrameworkBarrier">FrameworkBarrier</a>
1. [Usage](../pkg/barrier/barrier.go)
2. Example: [FrameworkBarrier Example](../example/framework/extension/frameworkbarrier.yaml), [TensorFlow Example](../example/framework/scenario/tensorflow), [etc](../example/framework/scenario).

## <a name="BestPractice">Best Practice</a>
[Best Practice](../pkg/apis/frameworkcontroller/v1/types.go)
