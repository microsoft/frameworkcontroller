### Run FrameworkController by a OS Process

Note at most one instance of FrameworkController can be run for a single k8s cluster.

```shell
export KUBE_APISERVER_ADDRESS={http[s]://host:port} && ./dist/frameworkcontroller/start.sh
```
