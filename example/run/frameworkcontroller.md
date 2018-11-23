# Run FrameworkController

1. Ensure at most one instance of FrameworkController is run for a single k8s cluster.
2. For the full FrameworkController configuration, see
 [Config Usage](../../pkg/apis/frameworkcontroller/v1/config.go) and [Config Example](../../example/config).

## Run by a OS Process

```shell
KUBE_APISERVER_ADDRESS={http[s]://host:port} ./dist/frameworkcontroller/start.sh
```
Or
```shell
KUBECONFIG={Process Local KubeConfig File Path} ./dist/frameworkcontroller/start.sh
```

## Run by a Docker Container

```shell
docker run -e KUBE_APISERVER_ADDRESS={http[s]://host:port} frameworkcontroller
```
Or
```shell
docker run -e KUBECONFIG={Container Local KubeConfig File Path} frameworkcontroller
```

## Run by a Kubernetes StatefulSet

```shell
kubectl create -f frameworkcontroller.yaml
```

frameworkcontroller.yaml:
```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: frameworkcontroller
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
      containers:
      - name: frameworkcontroller
        # Using official image to demonstrate this example.
        image: frameworkcontroller/frameworkcontroller
        env:
        # May not need to specify KUBE_APISERVER_ADDRESS or KUBECONFIG
        # if the target cluster to control is the cluster running the
        # StatefulSet.
        # See k8s inClusterConfig.
        - name: KUBE_APISERVER_ADDRESS
          value: {http[s]://host:port}
        - name: KUBECONFIG
          value: {Pod Local KubeConfig File Path}
```
