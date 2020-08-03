# [PyTorch Elastic Training](https://pytorch.org/elastic) On FrameworkController

## Feature
1. Support to [ScaleUp/ScaleDown with Strong Safety Guarantee](../../../../../doc/user-manual.md#FrameworkRescale)
2. Support to use whole cluster shared etcd or per-application dedicated etcd. If latter is used, the etcd will be automatically cleaned up when the whole FrameworkAttempt is completed
3. [Common Feature](../../../../../README.md#Feature)

## Prerequisite
1. Need to setup [Kubernetes DNS](https://kubernetes.io/docs/concepts/services-networking/dns-pod-service)
2. Need to setup [Kubernetes GPU Device Plugin](https://kubernetes.io/docs/tasks/manage-gpus/scheduling-gpus) for at least 4 NVIDIA GPUs
3. Need to setup [Kubernetes Cluster-Level Logging](https://kubernetes.io/docs/concepts/cluster-administration/logging), if you need to persist and expose the log for deleted Pod

## ImageNet Example
1. Create Service for etcd as below, so that etcd can be discovered by training workers:
```yaml
apiVersion: v1
kind: Service
metadata:
  name: pet-etcd
spec:
  selector:
    FC_FRAMEWORK_NAME: pet
    FC_TASKROLE_NAME: etcd
  ports:
  - targetPort: 2379
    port: 2379
```
2. Create Service for training worker as below, so that training workers can be discovered by each other:
```yaml
# See comments in ./example/framework/basic/servicestateful.yaml
apiVersion: v1
kind: Service
metadata:
  name: pet-worker
spec:
  clusterIP: None
  publishNotReadyAddresses: true
  selector:
    FC_FRAMEWORK_NAME: pet
    FC_TASKROLE_NAME: worker
```
3. [Create Framework](../../../../../doc/user-manual.md#CREATE_Framework) for training as below, and wait until all Tasks are AttemptRunning:
```yaml
apiVersion: frameworkcontroller.microsoft.com/v1
kind: Framework
metadata:
  name: pet
spec:
  executionType: Start
  retryPolicy:
    fancyRetryPolicy: true
    maxRetryCount: 2
  taskRoles:
  - name: etcd
    taskNumber: 1
    frameworkAttemptCompletionPolicy:
      minFailedTaskCount: 1
      minSucceededTaskCount: -1
    task:
      # Always retry etcd if there is only etcd failure
      retryPolicy:
        fancyRetryPolicy: false
        maxRetryCount: -2
      # Large timeout to force delete Pod as it may break the stateful batch.
      podGracefulDeletionTimeoutSec: 1800
      pod:
        spec:
          restartPolicy: Always
          containers:
          - name: etcd
            image: quay.io/coreos/etcd:v3.4.9
            command: [
            "sh", "-c",
            "/usr/local/bin/etcd
            --data-dir /var/lib/etcd --enable-v2
            --listen-client-urls http://0.0.0.0:2379
            --advertise-client-urls http://0.0.0.0:2379
            --initial-cluster-state new"]
            ports:
            - containerPort: 2379
  - name: worker
    # Should within torchelastic nnodes range.
    taskNumber: 4
    # As exit barrier is not yet supported in below torchelastic image, it is better
    # to still wait until all workers succeeded and only until then succeed the
    # whole training.
    frameworkAttemptCompletionPolicy:
      minFailedTaskCount: 1
      minSucceededTaskCount: 4
    task:
      retryPolicy:
        fancyRetryPolicy: true
        maxRetryCount: 2
      # Large timeout to force delete Pod as it may break the stateful batch.
      podGracefulDeletionTimeoutSec: 1800
      pod:
        spec:
          # See comments in ./example/framework/basic/servicestateful.yaml
          hostname: "{{FC_TASKROLE_NAME}}-{{FC_TASK_INDEX}}"
          subdomain: "{{FC_FRAMEWORK_NAME}}-{{FC_TASKROLE_NAME}}"
          restartPolicy: Never
          containers:
          - name: pytorch
            # Using official image to demonstrate this example.
            # The imagenet example does not require a distributed shared file system to broadcast checkpoint:
            # https://github.com/pytorch/elastic/blob/45dc33f3eca1344fe1fd84634fb0d62767822f3e/examples/imagenet/main.py#L336-L343
            image: torchelastic/examples:0.2.0
            command: [
              "sh", "-c",
              "python -m torchelastic.distributed.launch
              --rdzv_backend=etcd
              --rdzv_endpoint=${FC_FRAMEWORK_NAME}-etcd:2379
              --rdzv_id=${FC_FRAMEWORK_NAME}
              --nnodes=1:4
              --nproc_per_node=1
              /workspace/examples/imagenet/main.py
              --arch=resnet18 --batch-size=32 --epochs=2
              /workspace/data/tiny-imagenet-200"]
            resources:
              limits:
                # Should equal to torchelastic nproc_per_node.
                nvidia.com/gpu: 1
            volumeMounts:
            # Mount shared memory otherwise pytorch data loaders may be OOM.
            - name: shm-volume
              mountPath: /dev/shm
          volumes:
          - name: shm-volume
            emptyDir:
              medium: Memory
```
4. All workers will train the model, with log like below:
```text
[INFO] 2020-07-31 06:52:44,968 launch: Running torchelastic.distributed.launch with args: ['/opt/conda/lib/python3.7/site-packages/torchelastic/distributed/launch.py', '--rdzv_backend=etcd', '--rdzv_endpoint=pet-etcd:2379', '--rdzv_id=pet', '--nnodes=1:4', '--nproc_per_node=1', '/workspace/examples/imagenet/main.py', '--arch=resnet18', '--batch-size=32', '--epochs=2', '/workspace/data/tiny-imagenet-200']
INFO 2020-07-31 06:52:44,976 Etcd machines: ['http://0.0.0.0:2379']
[INFO] 2020-07-31 06:52:44,985 launch: Using nproc_per_node=1.
[INFO] 2020-07-31 06:52:45,715 api: [default] starting workers for function: wrapper_fn
[INFO] 2020-07-31 06:52:45,715 api: [default] Rendezvous'ing worker group
INFO 2020-07-31 06:52:45,715 Attempting to join next rendezvous
INFO 2020-07-31 06:52:45,723 New rendezvous state created: {'status': 'joinable', 'version': '1', 'participants': []}
INFO 2020-07-31 06:52:45,739 Joined rendezvous version 1 as rank 0. Full state: {'status': 'joinable', 'version': '1', 'participants': [0]}
INFO 2020-07-31 06:52:45,739 Rank 0 is responsible for join last call.
INFO 2020-07-31 06:52:46,942 Rank 0 finished join last call.
INFO 2020-07-31 06:52:46,942 Waiting for remaining peers.
INFO 2020-07-31 06:52:46,943 All peers arrived. Confirming membership.
INFO 2020-07-31 06:52:46,954 Waiting for confirmations from all peers.
INFO 2020-07-31 06:52:47,064 Rendezvous version 1 is complete. Final state: {'status': 'final', 'version': '1', 'participants': [0, 1, 2, 3], 'keep_alives': ['/torchelastic/p2p/run_pet/rdzv/v_1/rank_0', '/torchelastic/p2p/run_pet/rdzv/v_1/rank_3', '/torchelastic/p2p/run_pet/rdzv/v_1/rank_2', '/torchelastic/p2p/run_pet/rdzv/v_1/rank_1'], 'num_workers_waiting': 0}
INFO 2020-07-31 06:52:47,064 Creating EtcdStore as the c10d::Store implementation
[INFO] 2020-07-31 06:52:47,071 api: [default] Rendezvous complete for workers.
Result:
	restart_count=0
	group_rank=0
	group_world_size=4
	rank stride=1
	assigned global_ranks=[0]
	master_addr=worker-0.pet-worker.default.svc.cluster.local
	master_port=43429

[INFO] 2020-07-31 06:52:47,071 api: [default] Starting worker group
=> set cuda device = 0
=> creating model: resnet18
=> no workers have checkpoints, starting from epoch 0
=> start_epoch: 0, best_acc1: 0
Epoch: [0][  0/782]	Time  3.613 ( 3.613)	Data  0.079 ( 0.079)	Loss 7.0412e+00 (7.0412e+00)	Acc@1   0.00 (  0.00)	Acc@5   0.00 (  0.00)
Epoch: [0][ 10/782]	Time  1.638 ( 1.849)	Data  0.086 ( 0.300)	Loss 5.7640e+00 (6.3097e+00)	Acc@1   0.00 (  0.00)	Acc@5   0.00 (  1.99)
......
Test: [300/313]	Time  0.122 ( 0.167)	Loss 7.0159e+00 (7.0172e+00)	Acc@1   0.00 (  0.40)	Acc@5   6.25 (  1.43)
Test: [310/313]	Time  0.139 ( 0.166)	Loss 7.3541e+00 (7.0174e+00)	Acc@1   0.00 (  0.39)	Acc@5   3.12 (  1.43)
 * Acc@1 0.390 Acc@5 1.420
=> saved checkpoint for epoch 0 at /tmp/checkpoint.pth.tar
=> best model found at epoch 0 saving to /tmp/model_best.pth.tar
Epoch: [1][  0/782]	Time  6.522 ( 6.522)	Data  0.052 ( 0.052)	Loss 4.4326e+00 (4.4326e+00)	Acc@1   3.12 (  3.12)	Acc@5  15.62 ( 15.62)
Epoch: [1][ 10/782]	Time  1.427 ( 1.703)	Data  0.045 ( 0.202)	Loss 4.3480e+00 (4.4527e+00)	Acc@1   0.00 (  7.67)	Acc@5  34.38 ( 22.73)
......
```
5. [ScaleDown Framework](../../../../../doc/user-manual.md#Add_Delete_Task): Decrease `worker` taskNumber and minSucceededTaskCount from 4 to 2 by below patch:
```json
[
  {
    "op": "test",
    "path": "/spec/taskRoles/1/name",
    "value": "worker"
  },
  {
    "op": "replace",
    "path": "/spec/taskRoles/1/taskNumber",
    "value": 2
  },
  {
    "op": "replace",
    "path": "/spec/taskRoles/1/frameworkAttemptCompletionPolicy/minSucceededTaskCount",
    "value": 2
  }
]
```
6. Remaining workers `pet-worker-0`, `pet-worker-1` will re-rendezvous and recover from last epoch checkpoint, with log like below:
```text
......
Epoch: [1][180/782]	Time  1.186 ( 1.230)	Data  0.095 ( 0.179)	Loss 4.2262e+00 (4.3819e+00)	Acc@1   9.38 (  9.12)	Acc@5  34.38 ( 25.57)
Epoch: [1][190/782]	Time  1.580 ( 1.230)	Data  0.782 ( 0.180)	Loss 3.9395e+00 (4.3789e+00)	Acc@1   9.38 (  9.18)	Acc@5  34.38 ( 25.59)
Traceback (most recent call last):
  File "/workspace/examples/imagenet/main.py", line 603, in <module>
    main()
  File "/workspace/examples/imagenet/main.py", line 188, in main
    train(train_loader, model, criterion, optimizer, epoch, device_id, print_freq)
  File "/workspace/examples/imagenet/main.py", line 471, in train
    loss.backward()
  File "/opt/conda/lib/python3.7/site-packages/torch/tensor.py", line 198, in backward
    torch.autograd.backward(self, gradient, retain_graph, create_graph)
  File "/opt/conda/lib/python3.7/site-packages/torch/autograd/__init__.py", line 100, in backward
    allow_unreachable=True)  # allow_unreachable flag
RuntimeError: NCCL error: unhandled system error, NCCL version 2.4.8
[ERROR] 2020-07-31 07:15:34,783 local_elastic_agent: [default] Worker group failed
Traceback (most recent call last):
  File "/opt/conda/lib/python3.7/site-packages/torchelastic/agent/server/local_elastic_agent.py", line 190, in _monitor_workers
    if self._process_context.join(timeout=-1):
  File "/opt/conda/lib/python3.7/site-packages/torch/multiprocessing/spawn.py", line 119, in join
    raise Exception(msg)
Exception: 

-- Process 0 terminated with the following error:
Traceback (most recent call last):
  File "/opt/conda/lib/python3.7/site-packages/torch/multiprocessing/spawn.py", line 20, in _wrap
    fn(i, *args)
  File "/opt/conda/lib/python3.7/site-packages/torchelastic/agent/server/local_elastic_agent.py", line 79, in _wrap
    ret = fn(*args)
  File "/opt/conda/lib/python3.7/site-packages/torchelastic/distributed/launch.py", line 392, in wrapper_fn
    raise subprocess.CalledProcessError(returncode=process.returncode, cmd=cmd)
subprocess.CalledProcessError: Command '['/opt/conda/bin/python', '-u', '/workspace/examples/imagenet/main.py', '--arch=resnet18', '--batch-size=32', '--epochs=2', '/workspace/data/tiny-imagenet-200']' returned non-zero exit status 1.

[INFO] 2020-07-31 07:15:34,785 api: [default] Worker group FAILED. 3/3 attempts left; will restart worker group
[INFO] 2020-07-31 07:15:34,785 api: [default] Stopping worker group
[INFO] 2020-07-31 07:15:34,785 api: [default] Rendezvous'ing worker group
INFO 2020-07-31 07:15:34,785 Attempting to join next rendezvous
INFO 2020-07-31 07:15:34,791 Observed existing rendezvous state: {'status': 'joinable', 'version': '2', 'participants': [0]}
INFO 2020-07-31 07:15:34,826 Joined rendezvous version 2 as rank 1. Full state: {'status': 'joinable', 'version': '2', 'participants': [0, 1]}
INFO 2020-07-31 07:15:34,826 Waiting for remaining peers.
INFO 2020-07-31 07:16:04,896 All peers arrived. Confirming membership.
INFO 2020-07-31 07:16:04,904 Waiting for confirmations from all peers.
INFO 2020-07-31 07:16:04,908 Rendezvous version 2 is complete. Final state: {'status': 'final', 'version': '2', 'participants': [0, 1], 'keep_alives': ['/torchelastic/p2p/run_pet/rdzv/v_2/rank_1', '/torchelastic/p2p/run_pet/rdzv/v_2/rank_0'], 'num_workers_waiting': 0}
INFO 2020-07-31 07:16:04,908 Creating EtcdStore as the c10d::Store implementation
[INFO] 2020-07-31 07:16:04,915 api: [default] Rendezvous complete for workers.
Result:
	restart_count=1
	group_rank=1
	group_world_size=2
	rank stride=1
	assigned global_ranks=[1]
	master_addr=worker-1.pet-worker.default.svc.cluster.local
	master_port=55787

[INFO] 2020-07-31 07:16:04,915 api: [default] Starting worker group
=> set cuda device = 0
=> creating model: resnet18
=> loading checkpoint file: /tmp/checkpoint.pth.tar
=> loaded checkpoint file: /tmp/checkpoint.pth.tar
=> using checkpoint from rank: 1, max_epoch: 0
=> checkpoint broadcast size is: 93588276
/opt/conda/conda-bld/pytorch_1587428398394/work/torch/csrc/utils/tensor_numpy.cpp:141: UserWarning: The given NumPy array is not writeable, and PyTorch does not support non-writeable tensors. This means you can write to the underlying (supposedly non-writeable) NumPy array using the tensor. You may want to copy the array to protect its data or make it writeable before converting it to a tensor. This type of warning will be suppressed for the rest of this program.
=> done broadcasting checkpoint
=> done restoring from previous checkpoint
=> start_epoch: 1, best_acc1: 0.38999998569488525
Epoch: [1][   0/1563]	Time  2.916 ( 2.916)	Data  0.095 ( 0.095)	Loss 4.3512e+00 (4.3512e+00)	Acc@1  15.62 ( 15.62)	Acc@5  21.88 ( 21.88)
Epoch: [1][  10/1563]	Time  0.650 ( 0.833)	Data  0.043 ( 0.090)	Loss 4.4603e+00 (4.4707e+00)	Acc@1  12.50 (  8.52)	Acc@5  21.88 ( 20.45)
......
```
7. [ScaleUp Framework](../../../../../doc/user-manual.md#Add_Delete_Task): Increase `worker` taskNumber and minSucceededTaskCount from 2 to 3 by below patch:
```json
[
  {
    "op": "test",
    "path": "/spec/taskRoles/1/name",
    "value": "worker"
  },
  {
    "op": "replace",
    "path": "/spec/taskRoles/1/taskNumber",
    "value": 3
  },
  {
    "op": "replace",
    "path": "/spec/taskRoles/1/frameworkAttemptCompletionPolicy/minSucceededTaskCount",
    "value": 3
  }
]
```
8. Augmented workers `pet-worker-0`, `pet-worker-1`, `pet-worker-2` will re-rendezvous and recover from last epoch checkpoint, with log like below:
```text
......
Epoch: [1][1450/1563]	Time  0.563 ( 0.783)	Data  0.108 ( 0.177)	Loss 4.0248e+00 (4.2794e+00)	Acc@1  12.50 ( 10.54)	Acc@5  37.50 ( 28.52)
Epoch: [1][1460/1563]	Time  0.560 ( 0.783)	Data  0.074 ( 0.179)	Loss 4.5901e+00 (4.2787e+00)	Acc@1   6.25 ( 10.55)	Acc@5  18.75 ( 28.53)
[INFO] 2020-07-31 07:35:16,348 api: [default] Detected 1 new nodes from group_rank=1; will restart worker group
[INFO] 2020-07-31 07:35:16,349 api: [default] Stopping worker group
[INFO] 2020-07-31 07:35:21,306 api: [default] Rendezvous'ing worker group
INFO 2020-07-31 07:35:21,307 Attempting to join next rendezvous
INFO 2020-07-31 07:35:21,310 Observed existing rendezvous state: {'status': 'final', 'version': '2', 'participants': [0, 1], 'keep_alives': ['/torchelastic/p2p/run_pet/rdzv/v_2/rank_1', '/torchelastic/p2p/run_pet/rdzv/v_2/rank_0'], 'num_workers_waiting': 1}
INFO 2020-07-31 07:35:21,363 Announce self as waiting CAS unsuccessful, retrying
INFO 2020-07-31 07:35:21,411 Added self to waiting list. Rendezvous full state: {"status": "final", "version": "2", "participants": [0, 1], "keep_alives": ["/torchelastic/p2p/run_pet/rdzv/v_2/rank_1", "/torchelastic/p2p/run_pet/rdzv/v_2/rank_0"], "num_workers_waiting": 3}
INFO 2020-07-31 07:35:30,806 Keep-alive key /torchelastic/p2p/run_pet/rdzv/v_2/rank_1 is not renewed.
INFO 2020-07-31 07:35:30,807 Rendevous version 2 is incomplete. 
INFO 2020-07-31 07:35:30,807 Attempting to destroy it.
INFO 2020-07-31 07:35:30,808 Rendezvous attempt failed, will retry. Reason: Key not found : /torchelastic/p2p/run_pet/rdzv/active_version
INFO 2020-07-31 07:35:31,810 Attempting to join next rendezvous
INFO 2020-07-31 07:35:31,813 Observed existing rendezvous state: {'status': 'joinable', 'version': '3', 'participants': [0]}
INFO 2020-07-31 07:35:31,867 Joined rendezvous version 3 as rank 1. Full state: {'status': 'joinable', 'version': '3', 'participants': [0, 1]}
INFO 2020-07-31 07:35:31,868 Waiting for remaining peers.
INFO 2020-07-31 07:36:01,831 All peers arrived. Confirming membership.
INFO 2020-07-31 07:36:01,882 Waiting for confirmations from all peers.
INFO 2020-07-31 07:36:01,915 Rendezvous version 3 is complete. Final state: {'status': 'final', 'version': '3', 'participants': [0, 1, 2], 'keep_alives': ['/torchelastic/p2p/run_pet/rdzv/v_3/rank_0', '/torchelastic/p2p/run_pet/rdzv/v_3/rank_1', '/torchelastic/p2p/run_pet/rdzv/v_3/rank_2'], 'num_workers_waiting': 0}
INFO 2020-07-31 07:36:01,915 Creating EtcdStore as the c10d::Store implementation
[INFO] 2020-07-31 07:36:01,919 api: [default] Rendezvous complete for workers.
Result:
	restart_count=1
	group_rank=1
	group_world_size=3
	rank stride=1
	assigned global_ranks=[1]
	master_addr=worker-1.pet-worker.default.svc.cluster.local
	master_port=44823

[INFO] 2020-07-31 07:36:01,919 api: [default] Starting worker group
=> set cuda device = 0
=> creating model: resnet18
=> loading checkpoint file: /tmp/checkpoint.pth.tar
=> loaded checkpoint file: /tmp/checkpoint.pth.tar
=> using checkpoint from rank: 1, max_epoch: 0
=> checkpoint broadcast size is: 93588276
/opt/conda/conda-bld/pytorch_1587428398394/work/torch/csrc/utils/tensor_numpy.cpp:141: UserWarning: The given NumPy array is not writeable, and PyTorch does not support non-writeable tensors. This means you can write to the underlying (supposedly non-writeable) NumPy array using the tensor. You may want to copy the array to protect its data or make it writeable before converting it to a tensor. This type of warning will be suppressed for the rest of this program.
=> done broadcasting checkpoint
=> done restoring from previous checkpoint
=> start_epoch: 1, best_acc1: 0.38999998569488525
Epoch: [1][   0/1042]	Time  2.741 ( 2.741)	Data  0.073 ( 0.073)	Loss 4.7370e+00 (4.7370e+00)	Acc@1   0.00 (  0.00)	Acc@5  15.62 ( 15.62)
Epoch: [1][  10/1042]	Time  0.732 ( 1.339)	Data  0.045 ( 0.292)	Loss 4.5558e+00 (4.4728e+00)	Acc@1   3.12 (  7.39)	Acc@5  12.50 ( 23.86)
......
```
9. When training completed, all workers will succeed, with log like below:
```text
......
Test: [300/313]	Time  0.118 ( 0.179)	Loss 8.1714e+00 (8.1763e+00)	Acc@1   3.12 (  1.15)	Acc@5   6.25 (  3.87)
Test: [310/313]	Time  0.281 ( 0.177)	Loss 8.7371e+00 (8.1767e+00)	Acc@1   0.00 (  1.14)	Acc@5   6.25 (  3.86)
 * Acc@1 1.130 Acc@5 3.850
=> saved checkpoint for epoch 1 at /tmp/checkpoint.pth.tar
=> best model found at epoch 1 saving to /tmp/model_best.pth.tar
[INFO] 2020-07-31 07:55:16,413 api: [default] All workers successfully finished.
```
10. Finally, the whole Framework will succeed, with Status like below:
```yaml
apiVersion: frameworkcontroller.microsoft.com/v1
kind: Framework
metadata:
  creationTimestamp: '2020-07-31T06:52:25Z'
  generation: 43
  name: pet
  namespace: default
  resourceVersion: '35492058'
  selfLink: "/apis/frameworkcontroller.microsoft.com/v1/namespaces/default/frameworks/pet"
  uid: c3f9e3a1-314d-4b5f-94b9-9287d15ac5d6
spec:
  description: ''
  executionType: Start
  retryPolicy:
    fancyRetryPolicy: true
    maxRetryCount: 2
  taskRoles:
  - frameworkAttemptCompletionPolicy:
      minFailedTaskCount: 1
      minSucceededTaskCount: -1
    name: etcd
    task:
      pod:
        metadata:
          creationTimestamp: 
        spec:
          containers:
          - command:
            - sh
            - "-c"
            - "/usr/local/bin/etcd --data-dir /var/lib/etcd --enable-v2 --listen-client-urls
              http://0.0.0.0:2379 --advertise-client-urls http://0.0.0.0:2379 --initial-cluster-state
              new"
            image: quay.io/coreos/etcd:v3.4.9
            name: etcd
            ports:
            - containerPort: 2379
            resources: {}
          restartPolicy: Always
      podGracefulDeletionTimeoutSec: 1800
      retryPolicy:
        fancyRetryPolicy: false
        maxRetryCount: -2
    taskNumber: 1
  - frameworkAttemptCompletionPolicy:
      minFailedTaskCount: 1
      minSucceededTaskCount: 3
    name: worker
    task:
      pod:
        metadata:
          creationTimestamp: 
        spec:
          containers:
          - command:
            - sh
            - "-c"
            - python -m torchelastic.distributed.launch --rdzv_backend=etcd --rdzv_endpoint=${FC_FRAMEWORK_NAME}-etcd:2379
              --rdzv_id=${FC_FRAMEWORK_NAME} --nnodes=1:4 --nproc_per_node=1 /workspace/examples/imagenet/main.py
              --arch=resnet18 --batch-size=32 --epochs=2 /workspace/data/tiny-imagenet-200
            image: torchelastic/examples:0.2.0
            name: pytorch
            resources:
              limits:
                nvidia.com/gpu: '1'
            volumeMounts:
            - mountPath: "/dev/shm"
              name: shm-volume
          hostname: "{{FC_TASKROLE_NAME}}-{{FC_TASK_INDEX}}"
          restartPolicy: Never
          subdomain: "{{FC_FRAMEWORK_NAME}}-{{FC_TASKROLE_NAME}}"
          volumes:
          - emptyDir:
              medium: Memory
            name: shm-volume
      podGracefulDeletionTimeoutSec: 1800
      retryPolicy:
        fancyRetryPolicy: true
        maxRetryCount: 2
    taskNumber: 3
status:
  attemptStatus:
    completionStatus:
      code: 0
      diagnostics: Pod succeeded
      phrase: Succeeded
      trigger:
        message: SucceededTaskCount 3 has reached MinSucceededTaskCount 3 in the TaskRole
        taskIndex: 1
        taskRoleName: worker
      type:
        attributes: []
        name: Succeeded
    completionTime: '2020-07-31T07:56:14Z'
    configMapName: pet-attempt
    configMapUID: da3b7866-d1b2-45be-8222-ff85ac67ef23
    id: 0
    instanceUID: 0_da3b7866-d1b2-45be-8222-ff85ac67ef23
    runTime: '2020-07-31T06:52:30Z'
    startTime: '2020-07-31T06:52:25Z'
    taskRoleStatuses:
    - name: etcd
      podGracefulDeletionTimeoutSec: 1800
      taskStatuses:
      - attemptStatus:
          completionStatus:
            code: -220
            diagnostics: Stop to complete current FrameworkAttempt
            phrase: FrameworkAttemptCompletion
            type:
              attributes:
              - Permanent
              name: Failed
          completionTime: '2020-07-31T07:56:13Z'
          id: 0
          instanceUID: 0_7870a450-4eb4-4596-a0d2-be5c6727de03
          podHostIP: 10.151.40.232
          podIP: 10.207.128.5
          podName: pet-etcd-0
          podNodeName: node11
          podUID: 7870a450-4eb4-4596-a0d2-be5c6727de03
          runTime: '2020-07-31T06:52:30Z'
          startTime: '2020-07-31T06:52:25Z'
        completionTime: '2020-07-31T07:56:14Z'
        deletionPending: false
        index: 0
        retryPolicyStatus:
          accountableRetriedCount: 0
          retryDelaySec: 
          totalRetriedCount: 0
        startTime: '2020-07-31T06:52:25Z'
        state: Completed
        transitionTime: '2020-07-31T07:56:14Z'
    - name: worker
      podGracefulDeletionTimeoutSec: 1800
      taskStatuses:
      - attemptStatus:
          completionStatus:
            code: 0
            diagnostics: Pod succeeded
            phrase: Succeeded
            pod:
              containers:
              - code: 0
                name: pytorch
                reason: Completed
            type:
              attributes: []
              name: Succeeded
          completionTime: '2020-07-31T07:55:18Z'
          id: 0
          instanceUID: 0_a550a22a-6371-4ac3-991b-fc5957fb0dac
          podHostIP: 10.151.40.230
          podIP: 10.204.128.1
          podName: pet-worker-0
          podNodeName: node9
          podUID: a550a22a-6371-4ac3-991b-fc5957fb0dac
          runTime: '2020-07-31T06:52:30Z'
          startTime: '2020-07-31T06:52:25Z'
        completionTime: '2020-07-31T07:55:18Z'
        deletionPending: false
        index: 0
        retryPolicyStatus:
          accountableRetriedCount: 0
          retryDelaySec: 
          totalRetriedCount: 0
        startTime: '2020-07-31T06:52:25Z'
        state: Completed
        transitionTime: '2020-07-31T07:55:18Z'
      - attemptStatus:
          completionStatus:
            code: 0
            diagnostics: Pod succeeded
            phrase: Succeeded
            pod:
              containers:
              - code: 0
                name: pytorch
                reason: Completed
            type:
              attributes: []
              name: Succeeded
          completionTime: '2020-07-31T07:55:29Z'
          id: 0
          instanceUID: 0_6213facb-d11d-416d-9530-32adeb708439
          podHostIP: 10.151.40.231
          podIP: 10.201.0.2
          podName: pet-worker-1
          podNodeName: node10
          podUID: 6213facb-d11d-416d-9530-32adeb708439
          runTime: '2020-07-31T06:52:31Z'
          startTime: '2020-07-31T06:52:25Z'
        completionTime: '2020-07-31T07:55:30Z'
        deletionPending: false
        index: 1
        retryPolicyStatus:
          accountableRetriedCount: 0
          retryDelaySec: 
          totalRetriedCount: 0
        startTime: '2020-07-31T06:52:25Z'
        state: Completed
        transitionTime: '2020-07-31T07:55:30Z'
      - attemptStatus:
          completionStatus:
            code: 0
            diagnostics: Pod succeeded
            phrase: Succeeded
            pod:
              containers:
              - code: 0
                name: pytorch
                reason: Completed
            type:
              attributes: []
              name: Succeeded
          completionTime: '2020-07-31T07:55:22Z'
          id: 0
          instanceUID: 0_d12681fc-986d-4f36-95f3-f0edde5750ca
          podHostIP: 10.151.40.227
          podIP: 10.202.128.6
          podName: pet-worker-2
          podNodeName: node6
          podUID: d12681fc-986d-4f36-95f3-f0edde5750ca
          runTime: '2020-07-31T07:34:58Z'
          startTime: '2020-07-31T07:34:55Z'
        completionTime: '2020-07-31T07:55:22Z'
        deletionPending: false
        index: 2
        retryPolicyStatus:
          accountableRetriedCount: 0
          retryDelaySec: 
          totalRetriedCount: 0
        startTime: '2020-07-31T07:34:55Z'
        state: Completed
        transitionTime: '2020-07-31T07:55:22Z'
  completionTime: '2020-07-31T07:56:14Z'
  retryPolicyStatus:
    accountableRetriedCount: 0
    retryDelaySec: 
    totalRetriedCount: 0
  startTime: '2020-07-31T06:52:25Z'
  state: Completed
  transitionTime: '2020-07-31T07:56:14Z'
```
