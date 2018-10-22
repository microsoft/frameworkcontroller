# <a name="UserManual">User Manual</a>

## <a name="Index">Index</a>
   - [Architecture](#Architecture)
   - [Framework Interop](#FrameworkInterop)
   - [Container EnvironmentVariable](#ContainerEnvironmentVariable)
   - [CompletionCode Convention](#CompletionCodeConvention)
   - [RetryPolicy](#RetryPolicy)
   - [FrameworkAttemptCompletionPolicy](#FrameworkAttemptCompletionPolicy)
   - [Best Practice](#BestPractice)

## <a name="FrameworkInterop">Architecture</a>
<p style="text-align: left;">
  <img src="architecture.svg" title="Architecture" alt="Architecture" width="150%"/>
</p>


## <a name="FrameworkInterop">Framework Interop</a>
Supported interoperations with a Framework

| API Kind | Operations |
|:---- |:---- |
| Framework | [POST](/apis/frameworkcontroller.microsoft.com/v1/namespaces/{namespace}/frameworks) [GET](/apis/frameworkcontroller.microsoft.com/v1/namespaces/{namespace}/frameworks/{name}) [DELETE](/apis/frameworkcontroller.microsoft.com/v1/namespaces/{namespace}/frameworks/{name}) |
| ConfigMap | GET DELETE |
| Pod | GET DELETE |

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
