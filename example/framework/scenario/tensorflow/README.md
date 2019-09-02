# TensorFlow On FrameworkController

## Feature
1. Support both GPU and CPU Distributed Training
2. Automatically clean up PS when the whole FrameworkAttempt is completed
3. No need to adjust existing TensorFlow image
4. No need to setup [Kubernetes DNS](https://kubernetes.io/docs/concepts/services-networking/dns-pod-service) and [Kubernetes Service](https://kubernetes.io/docs/concepts/services-networking/service)
5. [Common Feature](../../../../README.md#Feature)

## Prerequisite
1. See `[PREREQUISITE]` in each specific Framework yaml file.
2. Need to setup [Kubernetes Cluster-Level Logging](https://kubernetes.io/docs/concepts/cluster-administration/logging), if you need to persist and expose the log for deleted Pod.

## Quick Start
1. [Common Quick Start](../../../../README.md#Quick-Start)
2. [CPU Example](cpu)
3. [GPU Example](gpu)
