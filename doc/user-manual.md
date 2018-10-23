# <a name="UserManual">User Manual</a>

## <a name="Index">Index</a>
   - [Framework Interop](#FrameworkInterop)
   - [Container EnvironmentVariable](#ContainerEnvironmentVariable)
   - [CompletionCode Convention](#CompletionCodeConvention)
   - [RetryPolicy](#RetryPolicy)
   - [FrameworkAttemptCompletionPolicy](#FrameworkAttemptCompletionPolicy)
   - [Best Practice](#BestPractice)

## <a name="FrameworkInterop">Framework Interop</a>
**Supported interoperations with a Framework**

| API Kind | Operations |
|:---- |:---- |
| Framework | [CREATE](#CREATE_Framework) [DELETE](#DELETE_Framework) [GET](#GET_Framework) [LIST](#LIST_Frameworks) [WATCH](#WATCH_Framework) [WATCH_LIST](#WATCH_LIST_Frameworks) |
| [ConfigMap](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#configmap-v1-core) | All operations except for [CREATE](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#create-193) [PUT](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#replace-195) [PATCH](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#patch-194) |
| [Pod](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#pod-v1-core) | All operations except for [CREATE](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#create-55) [PUT](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#replace-57) [PATCH](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#patch-56) |

**Supported clients to execute the interoperations with a Framework**

As Framework is actually a Kubernetes [CRD](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#customresourcedefinitions), all CRD clients can be used to execute the interoperations with a Framework, see them in [Accessing a custom resource](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#accessing-a-custom-resource).

### <a name="CREATE_Framework">CREATE Framework</a>
**Request**

    POST /apis/frameworkcontroller.microsoft.com/v1/namespaces/{FrameworkNamespace}/frameworks

Body: [Framework](../pkg/apis/frameworkcontroller/v1/types.go)

Type: application/json

**Description**

Create the specified Framework.

**Response**

| Code | Body | Description |
|:---- |:---- |:---- |
| OK(200) | [Framework](../pkg/apis/frameworkcontroller/v1/types.go) | Return current Framework. |
| Created(201) | [Framework](../pkg/apis/frameworkcontroller/v1/types.go) | Return current Framework. |
| Accepted(202) | [Framework](../pkg/apis/frameworkcontroller/v1/types.go) | Return current Framework. |
| Conflict(409) | [Status](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#status-v1-meta) | The specified Framework already exists. |

### <a name="DELETE_Framework">DELETE Framework</a>
**Request**

    DELETE /apis/frameworkcontroller.microsoft.com/v1/namespaces/{FrameworkNamespace}/frameworks/{FrameworkName}

Body:
```json
{
  "propagationPolicy": "Foreground"
}
```

Type: application/json

**Description**

Delete the specified Framework.

Notes:
* Should always use and only use the provided body, see [Framework Notes](../pkg/apis/frameworkcontroller/v1/types.go).

**Response**

| Code | Body | Description |
|:---- |:---- |:---- |
| OK(200) | [Framework](../pkg/apis/frameworkcontroller/v1/types.go) | The specified Framework is deleting.<br>Return current Framework. |
| OK(200) | [Status](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#status-v1-meta) | The specified Framework is deleted. |
| NotFound(200) | [Status](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#status-v1-meta) | The specified Framework is not found. |

### <a name="GET_Framework">GET Framework</a>
**Request**

    GET /apis/frameworkcontroller.microsoft.com/v1/namespaces/{FrameworkNamespace}/frameworks/{FrameworkName}

**Description**

Get the specified Framework.

**Response**

| Code | Body | Description |
|:---- |:---- |:---- |
| OK(200) | [Framework](../pkg/apis/frameworkcontroller/v1/types.go) | Return current Framework. |
| NotFound(200) | [Status](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#status-v1-meta) | The specified Framework is not found. |

### <a name="LIST_Frameworks">LIST Frameworks</a>
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

### <a name="WATCH_Framework">WATCH Framework</a>
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

### <a name="WATCH_LIST_Frameworks">WATCH_LIST Frameworks</a>
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
[CompletionCode Convention](../pkg/apis/frameworkcontroller/v1/types.go)

## <a name="RetryPolicy">RetryPolicy</a>
[RetryPolicy](../pkg/apis/frameworkcontroller/v1/types.go)

## <a name="FrameworkAttemptCompletionPolicy">FrameworkAttemptCompletionPolicy</a>
[FrameworkAttemptCompletionPolicy](../pkg/apis/frameworkcontroller/v1/types.go)

## <a name="BestPractice">Best Practice</a>
[Best Practice](../pkg/apis/frameworkcontroller/v1/types.go)
