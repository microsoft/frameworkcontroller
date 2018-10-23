### Run FrameworkController by a Kubernetes StatefulSet

Note at most one instance of FrameworkController can be run for a single Kubernetes cluster.

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
        # Using local image just to demonstrate this example.
        imagePullPolicy: Never
        image: frameworkcontroller
        env:
        # May not need to specify KUBE_APISERVER_ADDRESS if the target
        # cluster to control is the cluster running the StatefulSet.
        - name: KUBE_APISERVER_ADDRESS
          value: {http[s]://host:port}
```
