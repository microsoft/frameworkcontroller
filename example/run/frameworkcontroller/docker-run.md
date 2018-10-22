### Run FrameworkController by a Docker Container

Note at most one instance of FrameworkController can be run for a single k8s cluster.

```shell
docker run -e KUBE_APISERVER_ADDRESS={http[s]://host:port} frameworkcontroller
```
