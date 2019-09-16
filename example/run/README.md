# <a name="RunFrameworkController">Run FrameworkController</a>
We provide various approaches to run FrameworkController:
   - [Run By Kubernetes StatefulSet](#RunByKubernetesStatefulSet)
   - [Run By Docker Container](#RunByDockerContainer)
   - [Run By OS Process](#RunByOSProcess)

Notes:
   - For a single k8s cluster, one instance of FrameworkController orchestrates all Frameworks in all namespaces.
   - For a single k8s cluster, ensure at most one instance of FrameworkController is running at any point in time.
   - For the full FrameworkController configuration, see
 [Config Usage](../../pkg/apis/frameworkcontroller/v1/config.go) and [Config Example](../../example/config/default/frameworkcontroller.yaml).

## <a name="RunByKubernetesStatefulSet">Run By Kubernetes StatefulSet</a>
- This approach is better for production, since StatefulSet by itself provides [self-healing](https://kubernetes.io/docs/concepts/workloads/pods/pod/#durability-of-pods-or-lack-thereof) and can ensure [at most one instance](https://github.com/kubernetes/community/blob/ee8998b156031f6b363daade51ca2d12521f4ac0/contributors/design-proposals/storage/pod-safety.md) of FrameworkController is running at any point in time.
- Using official image to demonstrate this example.

**Prerequisite**

If the k8s cluster enforces [Authorization](https://kubernetes.io/docs/reference/access-authn-authz/authorization/#using-flags-for-your-authorization-module), you need to first create a [Service Account](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account) with granted permission for FrameworkController. For example, if the cluster enforces [RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#kubectl-create-clusterrolebinding):
```shell
kubectl create serviceaccount frameworkcontroller --namespace default
kubectl create clusterrolebinding frameworkcontroller \
  --clusterrole=cluster-admin \
  --user=system:serviceaccount:default:frameworkcontroller
```

**Run**

Run FrameworkController with above Service Account and the [k8s inClusterConfig](https://kubernetes.io/docs/tasks/access-application-cluster/access-cluster/#accessing-the-api-from-a-pod):
```shell
kubectl create -f frameworkcontroller.yaml
```

frameworkcontroller.yaml:
```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: frameworkcontroller
  namespace: default
spec:
  serviceName: frameworkcontroller
  selector:
    matchLabels:
      app: frameworkcontroller
  replicas: 1
  template:
    metadata:
      labels:
        app: frameworkcontroller
    spec:
      # Using the service account with granted permission
      # if the k8s cluster enforces authorization.
      serviceAccountName: frameworkcontroller
      containers:
      - name: frameworkcontroller
        image: frameworkcontroller/frameworkcontroller
        # Using k8s inClusterConfig, so usually, no need to specify
        # KUBE_APISERVER_ADDRESS or KUBECONFIG
        #env:
        #- name: KUBE_APISERVER_ADDRESS
        #  value: {http[s]://host:port}
        #- name: KUBECONFIG
        #  value: {Pod Local KubeConfig File Path}
```

## <a name="RunByDockerContainer">Run By Docker Container</a>
- This approach may be better for development sometimes.
- Using official image to demonstrate this example.

**Run**

If you have an insecure ApiServer address (can be got from [Insecure ApiServer](https://kubernetes.io/docs/reference/access-authn-authz/controlling-access/#api-server-ports-and-ips) or [kubectl proxy](https://kubernetes.io/docs/tasks/access-application-cluster/access-cluster/#using-kubectl-proxy)) which does not enforce authentication, you only need to provide the address:
```shell
docker run -e KUBE_APISERVER_ADDRESS={http[s]://host:port} \
  frameworkcontroller/frameworkcontroller
```

Otherwise, you need to provide your [KubeConfig File](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/#explore-the-home-kube-directory)  which inlines or refers the [ApiServer Credential Files](https://kubernetes.io/docs/reference/access-authn-authz/controlling-access/#transport-security) with [granted permission](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/#define-clusters-users-and-contexts):
```shell
docker run -e KUBECONFIG=/mnt/.kube/config \
  -v {Host Local KubeConfig File Path}:/mnt/.kube/config \
  -v {Host Local ApiServer Credential File Path}:{Container Local ApiServer Credential File Path} \
  frameworkcontroller/frameworkcontroller
```
For example, if the k8s cluster is created by [Minikube](https://kubernetes.io/docs/setup/minikube):
```shell
docker run -e KUBECONFIG=/mnt/.kube/config \
  -v ${HOME}/.kube/config:/mnt/.kube/config \
  -v ${HOME}/.minikube:${HOME}/.minikube \
  frameworkcontroller/frameworkcontroller
```

## <a name="RunByOSProcess">Run By OS Process</a>
- This approach may be better for development sometimes.
- Using local built binary distribution to demonstrate this example.

**Prerequisite**

Ensure you have installed [Golang 1.12.6 or above](https://golang.org/doc/install#install) and the [${GOPATH}](https://golang.org/doc/code.html#GOPATH) is valid.

Then build the FrameworkController binary distribution:
```shell
export PROJECT_DIR=${GOPATH}/src/github.com/microsoft/frameworkcontroller
rm -rf ${PROJECT_DIR}
mkdir -p ${PROJECT_DIR}
git clone https://github.com/Microsoft/frameworkcontroller.git ${PROJECT_DIR}
cd ${PROJECT_DIR}
./build/frameworkcontroller/go-build.sh
```

**Run**

If you have an insecure ApiServer address (can be got from [Insecure ApiServer](https://kubernetes.io/docs/reference/access-authn-authz/controlling-access/#api-server-ports-and-ips) or [kubectl proxy](https://kubernetes.io/docs/tasks/access-application-cluster/access-cluster/#using-kubectl-proxy)) which does not enforce authentication, you only need to provide the address:
```shell
KUBE_APISERVER_ADDRESS={http[s]://host:port} \
  ./dist/frameworkcontroller/start.sh
```

Otherwise, you need to provide your [KubeConfig File](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/#explore-the-home-kube-directory) which inlines or refers the [ApiServer Credential Files](https://kubernetes.io/docs/reference/access-authn-authz/controlling-access/#transport-security) with [granted permission](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/#define-clusters-users-and-contexts):
```shell
KUBECONFIG={Process Local KubeConfig File Path} \
  ./dist/frameworkcontroller/start.sh
```
For example:
```shell
KUBECONFIG=${HOME}/.kube/config \
  ./dist/frameworkcontroller/start.sh
```
And in above example, `${HOME}/.kube/config` is the default value of `KUBECONFIG`, so you can skip it:
```shell
./dist/frameworkcontroller/start.sh
```

## <a name="Next">Next</a>
1. [Submit Framework](../framework)
