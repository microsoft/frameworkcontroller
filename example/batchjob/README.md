# kube-batch 

kube-batch is a batch scheduler for Kubernetes, providing mechanisms for applications which would like to run batch jobs leveraging Kubernetes. It builds upon a decade and a half of experience on running batch workloads at scale using several systems, combined with best-of-breed ideas and practices from the open source community.   
1. [kube-batch](https://github.com/kubernetes-sigs/kube-batch)
2. [how to deploy kube-batch](https://github.com/kubernetes-sigs/kube-batch/blob/master/doc/usage/tutorial.md)

# FrameworkController how to use kube-batch

1. create a podgoup when create a framework, and set podgroup name to framework name, and the framework name is unique 
2. for gang-scheduling, need set podgroup minMember to Framework.Spec.MinMember, if it's <=0, set sum of all taskNumber
3. for queue-scheduling, need create a queue before, queue weight affects queue resources allocate 
4. for priority-scheduling, need create a PriorityClass, priority class weight affects scheduling sequence
5. when create pod, need set pod annotation "scheduling.k8s.io/group-name" value of a unique name,such as framework name, the same annotation belongs to the same job, if use priority-scheduling, check if Pod.Spec.Template.Spec.PriorityClassName is empty, set framework priorityClassName
6. last, when delete pod delete the podgroup

# Submit Framework
```bash
kubectl apply -f batchjob.yaml
```