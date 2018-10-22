// MIT License
//
// Copyright (c) Microsoft Corporation. All rights reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE

package controller

import (
	"fmt"
	"time"
	"reflect"
	"strings"
	log "github.com/sirupsen/logrus"
	errorWrap "github.com/pkg/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/types"
	core "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/rest"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	errorAgg "k8s.io/apimachinery/pkg/util/errors"
	kubeClient "k8s.io/client-go/kubernetes"
	kubeInformer "k8s.io/client-go/informers"
	coreLister "k8s.io/client-go/listers/core/v1"
	frameworkClient "github.com/microsoft/frameworkcontroller/pkg/client/clientset/versioned"
	frameworkInformer "github.com/microsoft/frameworkcontroller/pkg/client/informers/externalversions"
	frameworkLister "github.com/microsoft/frameworkcontroller/pkg/client/listers/frameworkcontroller/v1"
	ci "github.com/microsoft/frameworkcontroller/pkg/apis/frameworkcontroller/v1"
	"github.com/microsoft/frameworkcontroller/pkg/util"
	"github.com/microsoft/frameworkcontroller/pkg/common"
)

// FrameworkController maintains the lifecycle for all Frameworks in the cluster.
// It is the engine to transition the Framework.Status and other Framework related
// objects to satisfy the Framework.Spec eventually.
type FrameworkController struct {
	kConfig *rest.Config
	cConfig *ci.ControllerConfig

	// Client is used to write remote objects in ApiServer.
	// Remote objects are up-to-date and is writable.
	//
	// To read objects, it is better to use Lister instead of Client, since the
	// Lister is cached and the cache is the ground truth of other managed objects.
	//
	// Write remote objects cannot immediately change the local cached ground truth,
	// so, it is just a hint to drive the ground truth changes, and a complete write
	// should wait until the local cached objects reflect the write.
	//
	// Client already has retry policy to retry for most transient failures.
	// Client write failure does not mean the write does not succeed on remote, the
	// failure may be due to the success response is just failed to deliver to the
	// Client.
	kClient kubeClient.Interface
	fClient frameworkClient.Interface

	// Informer is used to sync remote objects to local cached objects, and deliver
	// events of object changes.
	//
	// The event delivery is level driven, not edge driven.
	// For example, the Informer may not deliver any event if a create is immediately
	// followed by a delete.
	cmInformer  cache.SharedIndexInformer
	podInformer cache.SharedIndexInformer
	fInformer   cache.SharedIndexInformer

	// Lister is used to read local cached objects in Informer.
	// Local cached objects may be outdated and is not writable.
	//
	// Outdated means current local cached objects may not reflect previous Client
	// remote writes.
	// For example, in previous round of syncFramework, Client created a Pod on
	// remote, however, in current round of syncFramework, the Pod may not appear
	// in the local cache, i.e. the local cached Pod is outdated.
	//
	// The local cached Framework.Status may be also outdated, so we take the
	// expected Framework.Status instead of the local cached one as the ground
	// truth of Framework.Status.
	//
	// The events of object changes are aligned with local cache, so we take the
	// local cached object instead of the remote one as the ground truth of
	// other managed objects except for the Framework.Status.
	// The outdated other managed object can be avoided by sync it only after the
	// remote write is also reflected in the local cache.
	cmLister  coreLister.ConfigMapLister
	podLister coreLister.PodLister
	fLister   frameworkLister.FrameworkLister

	// Queue is used to decouple items delivery and processing, i.e. control
	// how items are scheduled and distributed to process.
	// The items may come from Informer's events, or Controller's events, etc.
	//
	// It is not strictly FIFO because its Add method will only enqueue an item
	// if it is not already in the queue, i.e. the queue is deduplicated.
	// In fact, it is a FIFO pending set combined with a processing set instead of
	// a standard queue, i.e. a strict FIFO data structure.
	// So, even if we only allow to start a single worker, we cannot ensure all items
	// in the queue will be processed in FIFO order.
	// Finally, in any case, processing later enqueued item should not depend on the
	// result of processing previous enqueued item.
	//
	// However, it can be used to provide a processing lock for every different items
	// in the queue, i.e. the same item will not be processed concurrently, even in
	// the face of multiple concurrent workers.
	// Note, different items may still be processed concurrently in the face of
	// multiple concurrent workers. So, processing different items should modify
	// different objects to avoid additional concurrency control.
	//
	// Given above queue behaviors, we can choose to enqueue what kind of items:
	// 1. Framework Key
	//    Support multiple concurrent workers, but processing is coarse grained.
	//    Good at managing many small scale Frameworks.
	//    More idiomatic and easy to implement.
	// 2. All Managed Object Keys, such as Framework Key, Pod Key, etc
	//    Only support single worker, but processing is fine grained.
	//    Good at managing few large scale Frameworks.
	// 3. Events, such as [Pod p is added to Framework f]
	//    Only support single worker, and processing is fine grained.
	//    Good at managing few large scale Frameworks.
	// 4. Objects, such as Framework Object
	//    Only support single worker.
	//    Compared with local cached objects, the dequeued objects may be outdated.
	//    Internally, item will be used as map key, so objects means low performance.
	// Finally, we choose choice 1, so it is a Framework Key Queue.
	//
	// Processing is coarse grained:
	// Framework Key as item cannot differentiate Framework events, even for Add,
	// Update and Delete Framework event.
	// Besides, the dequeued item may be outdated compared the local cached one.
	// So, we can coarsen Add, Update and Delete event as a single Update event,
	// enqueue the Framework Key, and until the Framework Key is dequeued and started
	// to process, we refine the Update event to Add, Update or Delete event.
	//
	// Framework Key in queue should be valid, i.e. it can be SplitKey successfully.
	//
	// Enqueue a Framework Key means schedule a syncFramework for the Framework,
	// no matter the Framework's objects changed or not.
	//
	// Methods:
	// Add:
	//   Only keep the earliest item to dequeue:
	//   The item will only be enqueued if it is not already in the queue.
	// AddAfter:
	//   Only keep the earliest item to Add:
	//   The item may be Added before the duration elapsed, such as the same item
	//   is AddedAfter later with an earlier duration.
	fQueue workqueue.RateLimitingInterface

	// fExpectedStatusInfos is used to store the expected Framework.Status info for
	// all Frameworks.
	// See ExpectedFrameworkStatusInfo.
	//
	// Framework Key -> The expected Framework.Status info
	fExpectedStatusInfos map[string]*ExpectedFrameworkStatusInfo
}

type ExpectedFrameworkStatusInfo struct {
	// The expected Framework.Status.
	// It is the ground truth Framework.Status that the remote and the local cached
	// Framework.Status are expected to be.
	//
	// It is used to sync against the local cached Framework.Spec and the local
	// cached other related objects, and it helps to ensure the Framework.Status is
	// Monotonically Exposed.
	// Note, the local cached Framework.Status may be outdated compared with the
	// remote one, so without the it, the local cached Framework.Status is not
	// enough to ensure the Framework.Status is Monotonically Exposed.
	// See FrameworkStatus.
	status *ci.FrameworkStatus

	// Whether the expected Framework.Status is the same as the remote one.
	// It helps to ensure the expected Framework.Status is persisted before sync.
	remoteSynced bool
}

func NewQueueFrameworkController() *FrameworkController {
	log.Infof("Initializing " + ci.ControllerName)

	cConfig := ci.NewControllerConfig()
	common.LogLines("With ControllerConfig: \n%v", common.ToYaml(cConfig))
	kConfig := util.BuildKubeConfig(cConfig)

	kClient, fClient := util.CreateClients(kConfig)

	// Informer resync will periodically replay the event of all objects stored in its cache.
	// However, by design, Informer and Controller should not miss any event.
	// So, we should disable resync to avoid hiding missing event bugs inside Controller.
	//
	// TODO: Add AttemptCreating state after SharedInformer supports IncludeUninitialized.
	// So that we can move the object initialization time out of the
	// ObjectLocalCacheCreationTimeoutSec, to reduce the expectation timeout false alarm
	// rate when Pod is specified with Initializers.
	// See https://github.com/kubernetes/kubernetes/pull/51247
	cmListerInformer := kubeInformer.NewSharedInformerFactory(kClient, 0).Core().V1().ConfigMaps()
	podListerInformer := kubeInformer.NewSharedInformerFactory(kClient, 0).Core().V1().Pods()
	fListerInformer := frameworkInformer.NewSharedInformerFactory(fClient, 0).Frameworkcontroller().V1().Frameworks()
	cmInformer := cmListerInformer.Informer()
	podInformer := podListerInformer.Informer()
	fInformer := fListerInformer.Informer()
	cmLister := cmListerInformer.Lister()
	podLister := podListerInformer.Lister()
	fLister := fListerInformer.Lister()

	// Using DefaultControllerRateLimiter to rate limit on both particular items and overall items.
	fQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	c := &FrameworkController{
		kConfig:              kConfig,
		cConfig:              cConfig,
		kClient:              kClient,
		fClient:              fClient,
		cmInformer:           cmInformer,
		podInformer:          podInformer,
		fInformer:            fInformer,
		cmLister:             cmLister,
		podLister:            podLister,
		fLister:              fLister,
		fQueue:               fQueue,
		fExpectedStatusInfos: map[string]*ExpectedFrameworkStatusInfo{},
	}

	fInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueFrameworkObj(obj, "Framework Added")
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			// FrameworkController only cares about Framework.Spec update
			oldF := oldObj.(*ci.Framework)
			newF := newObj.(*ci.Framework)
			if !reflect.DeepEqual(oldF.Spec, newF.Spec) {
				c.enqueueFrameworkObj(newObj, "Framework.Spec Updated")
			}
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueFrameworkObj(obj, "Framework Deleted")
		},
	})

	cmInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueFrameworkConfigMapObj(obj, "Framework ConfigMap Added")
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.enqueueFrameworkConfigMapObj(newObj, "Framework ConfigMap Updated")
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueFrameworkConfigMapObj(obj, "Framework ConfigMap Deleted")
		},
	})

	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueFrameworkPodObj(obj, "Framework Pod Added")
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.enqueueFrameworkPodObj(newObj, "Framework Pod Updated")
		},
		DeleteFunc: func(obj interface{}) {
			c.enqueueFrameworkPodObj(obj, "Framework Pod Deleted")
		},
	})

	return c
}

// obj could be *ci.Framework or cache.DeletedFinalStateUnknown.
func (c *FrameworkController) enqueueFrameworkObj(obj interface{}, msg string) {
	key, err := util.GetKey(obj)
	if err != nil {
		log.Errorf("Failed to get key for obj %#v, skip to enqueue: %v", obj, err)
		return
	}

	_, _, err = util.SplitKey(key)
	if err != nil {
		log.Errorf("Got invalid key %v for obj %#v, skip to enqueue: %v", key, obj, err)
		return
	}

	c.fQueue.Add(key)
	log.Infof("[%v]: enqueueFrameworkObj: %v", key, msg)
}

// obj could be *core.ConfigMap or cache.DeletedFinalStateUnknown.
func (c *FrameworkController) enqueueFrameworkConfigMapObj(obj interface{}, msg string) {
	if cm := util.ToConfigMap(obj); cm != nil {
		if f := c.getConfigMapOwner(cm); f != nil {
			c.enqueueFrameworkObj(f, msg+": "+cm.Name)
		}
	}
}

// obj could be *core.Pod or cache.DeletedFinalStateUnknown.
func (c *FrameworkController) enqueueFrameworkPodObj(obj interface{}, msg string) {
	if pod := util.ToPod(obj); pod != nil {
		if cm := c.getPodOwner(pod); cm != nil {
			c.enqueueFrameworkConfigMapObj(cm, msg+": "+pod.Name)
		}
	}
}

func (c *FrameworkController) getConfigMapOwner(cm *core.ConfigMap) *ci.Framework {
	cmOwner := meta.GetControllerOf(cm)
	if cmOwner == nil {
		return nil
	}

	if cmOwner.Kind != ci.FrameworkKind {
		return nil
	}

	f, err := c.fLister.Frameworks(cm.Namespace).Get(cmOwner.Name)
	if err != nil {
		if !apiErrors.IsNotFound(err) {
			log.Errorf(
				"[%v]: ConfigMapOwner %#v cannot be got from local cache: %v",
				cm.Namespace+"/"+cm.Name, *cmOwner, err)
		}
		return nil
	}

	if f.UID != cmOwner.UID {
		// GarbageCollectionController will handle the dependent object
		// deletion according to the ownerReferences.
		return nil
	}

	return f
}

func (c *FrameworkController) getPodOwner(pod *core.Pod) *core.ConfigMap {
	podOwner := meta.GetControllerOf(pod)
	if podOwner == nil {
		return nil
	}

	if podOwner.Kind != ci.ConfigMapKind {
		return nil
	}

	cm, err := c.cmLister.ConfigMaps(pod.Namespace).Get(podOwner.Name)
	if err != nil {
		if !apiErrors.IsNotFound(err) {
			log.Errorf(
				"[%v]: PodOwner %#v cannot be got from local cache: %v",
				pod.Namespace+"/"+pod.Name, *podOwner, err)
		}
		return nil
	}

	if cm.UID != podOwner.UID {
		// GarbageCollectionController will handle the dependent object
		// deletion according to the ownerReferences.
		return nil
	}

	return cm
}

func (c *FrameworkController) Run(stopCh <-chan struct{}) {
	defer c.fQueue.ShutDown()
	defer log.Errorf("Stopping " + ci.ControllerName)
	defer runtime.HandleCrash()

	log.Infof("Recovering " + ci.ControllerName)
	util.PutCRD(
		c.kConfig,
		ci.BuildFrameworkCRD(),
		c.cConfig.CRDEstablishedCheckIntervalSec,
		c.cConfig.CRDEstablishedCheckTimeoutSec)

	go c.fInformer.Run(stopCh)
	go c.cmInformer.Run(stopCh)
	go c.podInformer.Run(stopCh)
	if !cache.WaitForCacheSync(
		stopCh,
		c.fInformer.HasSynced,
		c.cmInformer.HasSynced,
		c.podInformer.HasSynced) {
		panic("Failed to WaitForCacheSync")
	}

	log.Infof("Running %v with %v workers",
		ci.ControllerName, *c.cConfig.WorkerNumber)

	for i := int32(0); i < *c.cConfig.WorkerNumber; i++ {
		// id is dedicated for each iteration, while i is not.
		id := i
		go wait.Until(func() { c.worker(id) }, time.Second, stopCh)
	}

	<-stopCh
}

func (c *FrameworkController) worker(id int32) {
	defer log.Errorf("Stopping worker-%v", id)
	log.Infof("Running worker-%v", id)

	for c.processNextWorkItem(id) {
	}
}

func (c *FrameworkController) processNextWorkItem(id int32) bool {
	// Blocked to get an item which is different from the current processing items.
	key, quit := c.fQueue.Get()
	if quit {
		return false
	}
	log.Infof("[%v]: Assigned to worker-%v", key, id)

	// Remove the item from the current processing items to unblock getting the
	// same item again.
	defer c.fQueue.Done(key)

	err := c.syncFramework(key.(string))
	if err == nil {
		// Reset the rate limit counters of the item in the queue, such as NumRequeues,
		// because we have synced it successfully.
		c.fQueue.Forget(key)
	} else {
		c.fQueue.AddRateLimited(key)
	}

	return true
}

// It should not be invoked concurrently with the same key.
//
// Return error only for Platform Transient Error, so that the key
// can be enqueued again after rate limited delay.
// For Platform Permanent Error, it should be delivered by panic.
// For Framework Error, it should be delivered into Framework.Status.
func (c *FrameworkController) syncFramework(key string) (returnedErr error) {
	startTime := time.Now()
	logPfx := fmt.Sprintf("[%v]: syncFramework: ", key)
	log.Infof(logPfx + "Started")
	defer func() {
		if returnedErr != nil {
			// returnedErr is already prefixed with logPfx
			log.Warnf(returnedErr.Error())
			log.Warnf(logPfx +
					"Failed to due to Platform Transient Error. " +
					"Will enqueue it again after rate limited delay")
		}
		log.Infof(logPfx+"Completed: Duration %v", time.Since(startTime))
	}()

	namespace, name, err := util.SplitKey(key)
	if err != nil {
		// Unreachable
		panic(fmt.Errorf(logPfx+
				"Failed: Got invalid key from queue, but the queue should only contain "+
				"valid keys: %v", err))
	}

	localF, err := c.fLister.Frameworks(namespace).Get(name)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			// GarbageCollectionController will handle the dependent object
			// deletion according to the ownerReferences.
			log.Infof(logPfx+
					"Skipped: Framework cannot be found in local cache: %v", err)
			c.deleteExpectedFrameworkStatusInfo(key)
			return nil
		} else {
			return fmt.Errorf(logPfx+
					"Failed: Framework cannot be got from local cache: %v", err)
		}
	} else {
		if localF.DeletionTimestamp != nil {
			// Skip syncFramework to avoid fighting with GarbageCollectionController,
			// because GarbageCollectionController may be deleting the dependent object.
			log.Infof(logPfx+
					"Skipped: Framework is deleting: Will be deleted at %v",
				localF.DeletionTimestamp)
			return nil
		} else {
			f := localF.DeepCopy()
			// From now on, f is a writable copy of the original local cached one, and
			// it may be different from the original one.

			expected, exists := c.getExpectedFrameworkStatusInfo(f.Key())
			if !exists {
				if f.Status != nil {
					// Recover f related things, since it is the first time we see it and
					// its Status is not nil.
					c.recoverFrameworkWorkItems(f)
				}

				// f.Status must be the same as the remote one, since it is the first
				// time we see it.
				c.updateExpectedFrameworkStatusInfo(f.Key(), f.Status, true)
			} else {
				// f.Status may be outdated, so override it with the expected one, to
				// ensure the Framework.Status is Monotonically Exposed.
				f.Status = expected.status

				// Ensure the expected Framework.Status is the same as the remote one
				// before sync.
				if !expected.remoteSynced {
					updateErr := c.updateRemoteFrameworkStatus(f)
					if updateErr != nil {
						return updateErr
					}
					c.updateExpectedFrameworkStatusInfo(f.Key(), f.Status, true)
				}
			}

			// At this point, f.Status is the same as the expected and remote
			// Framework.Status, so it is ready to sync against f.Spec and other
			// related objects.
			errs := []error{}
			remoteF := f.DeepCopy()

			syncErr := c.syncFrameworkStatus(f)
			errs = append(errs, syncErr)

			if !reflect.DeepEqual(remoteF.Status, f.Status) {
				// Always update the expected and remote Framework.Status even if sync
				// error, since f.Status should never be corrupted due to any Platform
				// Transient Error, so no need to rollback to the one before sync, and
				// no need to DeepCopy between f.Status and the expected one.
				updateErr := c.updateRemoteFrameworkStatus(f)
				errs = append(errs, updateErr)

				c.updateExpectedFrameworkStatusInfo(f.Key(), f.Status, updateErr == nil)
			} else {
				log.Infof(logPfx +
						"Skip to update the expected and remote Framework.Status since " +
						"they are unchanged")
			}

			return errorAgg.NewAggregate(errs)
		}
	}
}

// No need to recover the non-AddAfter items, because the Informer has already
// delivered the Add events for all recovered Frameworks which caused all
// Frameworks will be enqueued to sync.
func (c *FrameworkController) recoverFrameworkWorkItems(f *ci.Framework) {
	logPfx := fmt.Sprintf("[%v]: recoverFrameworkWorkItems: ", f.Key())
	log.Infof(logPfx + "Started")
	defer func() { log.Infof(logPfx + "Completed") }()

	if f.Status == nil {
		return
	}

	c.recoverTimeoutChecks(f)
}

func (c *FrameworkController) recoverTimeoutChecks(f *ci.Framework) {
	// If a check is already timeout, the timeout will be handled by the following
	// sync after the recover, so no need to enqueue it again.
	c.enqueueFrameworkAttemptCreationTimeoutCheck(f, true)
	c.enqueueFrameworkRetryDelayTimeoutCheck(f, true)
	for _, taskRoleStatus := range f.TaskRoleStatuses() {
		for _, taskStatus := range taskRoleStatus.TaskStatuses {
			taskRoleName := taskRoleStatus.Name
			taskIndex := taskStatus.Index
			c.enqueueTaskAttemptCreationTimeoutCheck(f, taskRoleName, taskIndex, true)
			c.enqueueTaskRetryDelayTimeoutCheck(f, taskRoleName, taskIndex, true)
		}
	}
}

func (c *FrameworkController) enqueueFrameworkAttemptCreationTimeoutCheck(
		f *ci.Framework, failIfTimeout bool) bool {
	if f.Status.State != ci.FrameworkAttemptCreationRequested {
		return false
	}

	leftDuration := common.CurrentLeftDuration(
		f.Status.TransitionTime,
		c.cConfig.ObjectLocalCacheCreationTimeoutSec)
	if common.IsTimeout(leftDuration) && failIfTimeout {
		return false
	}

	c.fQueue.AddAfter(f.Key(), leftDuration)
	log.Infof("[%v]: enqueueFrameworkAttemptCreationTimeoutCheck after %v",
		f.Key(), leftDuration)
	return true
}

func (c *FrameworkController) enqueueTaskAttemptCreationTimeoutCheck(
		f *ci.Framework, taskRoleName string, taskIndex int32,
		failIfTimeout bool) bool {
	taskStatus := f.TaskStatus(taskRoleName, taskIndex)
	if taskStatus.State != ci.TaskAttemptCreationRequested {
		return false
	}

	leftDuration := common.CurrentLeftDuration(
		taskStatus.TransitionTime,
		c.cConfig.ObjectLocalCacheCreationTimeoutSec)
	if common.IsTimeout(leftDuration) && failIfTimeout {
		return false
	}

	c.fQueue.AddAfter(f.Key(), leftDuration)
	log.Infof("[%v][%v][%v]: enqueueTaskAttemptCreationTimeoutCheck after %v",
		f.Key(), taskRoleName, taskIndex, leftDuration)
	return true
}

func (c *FrameworkController) enqueueFrameworkRetryDelayTimeoutCheck(
		f *ci.Framework, failIfTimeout bool) bool {
	if f.Status.State != ci.FrameworkAttemptCompleted {
		return false
	}

	leftDuration := common.CurrentLeftDuration(
		f.Status.TransitionTime,
		f.Status.RetryPolicyStatus.RetryDelaySec)
	if common.IsTimeout(leftDuration) && failIfTimeout {
		return false
	}

	c.fQueue.AddAfter(f.Key(), leftDuration)
	log.Infof("[%v]: enqueueFrameworkRetryDelayTimeoutCheck after %v",
		f.Key(), leftDuration)
	return true
}

func (c *FrameworkController) enqueueTaskRetryDelayTimeoutCheck(
		f *ci.Framework, taskRoleName string, taskIndex int32,
		failIfTimeout bool) bool {
	taskStatus := f.TaskStatus(taskRoleName, taskIndex)
	if taskStatus.State != ci.TaskAttemptCompleted {
		return false
	}

	leftDuration := common.CurrentLeftDuration(
		taskStatus.TransitionTime,
		taskStatus.RetryPolicyStatus.RetryDelaySec)
	if common.IsTimeout(leftDuration) && failIfTimeout {
		return false
	}

	c.fQueue.AddAfter(f.Key(), leftDuration)
	log.Infof("[%v][%v][%v]: enqueueTaskRetryDelayTimeoutCheck after %v",
		f.Key(), taskRoleName, taskIndex, leftDuration)
	return true
}

func (c *FrameworkController) enqueueFramework(f *ci.Framework, msg string) {
	c.fQueue.Add(f.Key())
	log.Infof("[%v]: enqueueFramework: %v", f.Key(), msg)
}

func (c *FrameworkController) syncFrameworkStatus(f *ci.Framework) error {
	logPfx := fmt.Sprintf("[%v]: syncFrameworkStatus: ", f.Key())
	log.Infof(logPfx + "Started")
	defer func() { log.Infof(logPfx + "Completed") }()

	if f.Status == nil {
		f.Status = f.NewFrameworkStatus()
	} else {
		// TODO: Support Framework.Spec Update
	}

	return c.syncFrameworkState(f)
}

func (c *FrameworkController) syncFrameworkState(f *ci.Framework) error {
	logPfx := fmt.Sprintf("[%v]: syncFrameworkState: ", f.Key())
	log.Infof(logPfx + "Started")
	defer func() { log.Infof(logPfx + "Completed") }()

	if f.Status.State == ci.FrameworkCompleted {
		log.Infof(logPfx + "Skipped: Framework is already completed")
		return nil
	}

	// Get the ground truth readonly cm
	cm, err := c.getOrCleanupConfigMap(f)
	if err != nil {
		return err
	}

	// Totally reconstruct FrameworkState in case Framework.Status is failed to
	// persist due to FrameworkController restart.
	if cm == nil {
		if f.ConfigMapUID() == nil {
			f.TransitionFrameworkState(ci.FrameworkAttemptCreationPending)
		} else {
			// Avoid sync with outdated object:
			// cm is remote creation requested but not found in the local cache.
			if f.Status.State == ci.FrameworkAttemptCreationRequested {
				if c.enqueueFrameworkAttemptCreationTimeoutCheck(f, true) {
					log.Infof(logPfx +
							"Waiting ConfigMap to appear in the local cache or timeout")
					return nil
				}

				diag := fmt.Sprintf(
					"ConfigMap does not appear in the local cache within timeout %v, "+
							"so consider it was deleted and force delete it",
					common.SecToDuration(c.cConfig.ObjectLocalCacheCreationTimeoutSec))
				log.Warnf(logPfx + diag)

				// Ensure cm is deleted in remote to avoid managed cm leak after
				// FrameworkCompleted.
				err := c.deleteConfigMap(f, *f.ConfigMapUID())
				if err != nil {
					return err
				}

				f.Status.AttemptStatus.CompletionStatus =
						ci.CompletionCodeConfigMapCreationTimeout.NewCompletionStatus(diag)
			}

			if f.Status.State != ci.FrameworkAttemptCompleted {
				if f.Status.AttemptStatus.CompletionStatus == nil {
					diag := fmt.Sprintf("ConfigMap was deleted by others")
					log.Warnf(logPfx + diag)
					f.Status.AttemptStatus.CompletionStatus =
							ci.CompletionCodeConfigMapExternalDeleted.NewCompletionStatus(diag)
				}

				f.Status.AttemptStatus.CompletionTime = common.PtrNow()
				f.TransitionFrameworkState(ci.FrameworkAttemptCompleted)
				log.Infof(logPfx+
						"FrameworkAttemptInstance %v is completed with CompletionStatus: %v",
					*f.FrameworkAttemptInstanceUID(),
					f.Status.AttemptStatus.CompletionStatus)
			}
		}
	} else {
		if cm.DeletionTimestamp == nil {
			if f.Status.State == ci.FrameworkAttemptDeletionPending {
				// The CompletionStatus has been persisted, so it is safe to delete the
				// cm now.
				err := c.deleteConfigMap(f, *f.ConfigMapUID())
				if err != nil {
					return err
				}
				f.TransitionFrameworkState(ci.FrameworkAttemptDeletionRequested)
			}

			// Avoid sync with outdated object:
			// cm is remote deletion requested but not deleting or deleted in the local
			// cache.
			if f.Status.State == ci.FrameworkAttemptDeletionRequested {
				// The deletion requested object will never appear again with the same UID,
				// so always just wait.
				log.Infof(logPfx +
						"Waiting ConfigMap to disappearing or disappear in the local cache")
				return nil
			}

			f.TransitionFrameworkState(ci.FrameworkAttemptRunning)
		} else {
			f.TransitionFrameworkState(ci.FrameworkAttemptDeleting)
			log.Infof(logPfx + "Waiting ConfigMap to be deleted")
			return nil
		}
	}
	// At this point, f.Status.State must be in:
	// {FrameworkAttemptCreationPending, FrameworkAttemptCompleted, FrameworkAttemptRunning}

	if f.Status.State == ci.FrameworkAttemptCompleted {
		// attemptToRetryFramework
		retryDecision := f.Spec.RetryPolicy.ShouldRetry(
			f.Status.RetryPolicyStatus,
			f.Status.AttemptStatus.CompletionStatus.Type,
			*c.cConfig.FrameworkMinRetryDelaySecForTransientConflictFailed,
			*c.cConfig.FrameworkMaxRetryDelaySecForTransientConflictFailed)

		if f.Status.RetryPolicyStatus.RetryDelaySec == nil {
			// RetryFramework is not yet scheduled, so need to be decided.
			if retryDecision.ShouldRetry {
				// scheduleToRetryFramework
				log.Infof(logPfx+
						"Will retry Framework with new FrameworkAttempt: RetryDecision: %v",
					retryDecision)

				f.Status.RetryPolicyStatus.RetryDelaySec = &retryDecision.DelaySec
			} else {
				// completeFramework
				log.Infof(logPfx+
						"Will complete Framework: RetryDecision: %v",
					retryDecision)

				f.Status.CompletionTime = common.PtrNow()
				f.TransitionFrameworkState(ci.FrameworkCompleted)
				return nil
			}
		}

		if f.Status.RetryPolicyStatus.RetryDelaySec != nil {
			// RetryFramework is already scheduled, so just need to check timeout.
			if c.enqueueFrameworkRetryDelayTimeoutCheck(f, true) {
				log.Infof(logPfx + "Waiting Framework to retry after delay")
				return nil
			}

			// retryFramework
			log.Infof(logPfx + "Retry Framework")
			f.Status.RetryPolicyStatus.TotalRetriedCount++
			if retryDecision.IsAccountable {
				f.Status.RetryPolicyStatus.AccountableRetriedCount++
			}
			f.Status.RetryPolicyStatus.RetryDelaySec = nil
			f.Status.AttemptStatus = f.NewFrameworkAttemptStatus(
				f.Status.RetryPolicyStatus.TotalRetriedCount)
			f.TransitionFrameworkState(ci.FrameworkAttemptCreationPending)
		}
	}
	// At this point, f.Status.State must be in:
	// {FrameworkAttemptCreationPending, FrameworkAttemptRunning}

	if f.Status.State == ci.FrameworkAttemptCreationPending {
		// createFrameworkAttempt
		cm, err := c.createConfigMap(f)
		if err != nil {
			return err
		}

		f.Status.AttemptStatus.ConfigMapUID = &cm.UID
		f.Status.AttemptStatus.InstanceUID = ci.GetFrameworkAttemptInstanceUID(
			f.FrameworkAttemptID(), f.ConfigMapUID())
		f.TransitionFrameworkState(ci.FrameworkAttemptCreationRequested)

		// Informer may not deliver any event if a create is immediately followed by
		// a delete, so manually enqueue a sync to check the cm existence after the
		// timeout.
		c.enqueueFrameworkAttemptCreationTimeoutCheck(f, false)

		// The ground truth cm is the local cached one instead of the remote one,
		// so need to wait before continue the sync.
		log.Infof(logPfx +
				"Waiting ConfigMap to appear in the local cache or timeout")
		return nil
	}
	// At this point, f.Status.State must be in:
	// {FrameworkAttemptRunning}

	if f.Status.State == ci.FrameworkAttemptRunning {
		// Only sync child Tasks when FrameworkAttemptRunning
		return c.syncTaskRoleStatuses(f, cm)
	} else {
		// Unreachable
		panic(fmt.Errorf(logPfx+
				"Failed: At this point, FrameworkState should be in {%v} instead of %v",
			ci.FrameworkAttemptRunning, f.Status.State))
	}
}

// Get Framework's current ConfigMap object, if not found, then clean up existing
// controlled ConfigMap if any.
// Returned cm is either managed or nil, if it is the managed cm, it is not
// writable and may be outdated even if no error.
// Clean up instead of recovery is because the ConfigMapUID is always the ground
// truth.
func (c *FrameworkController) getOrCleanupConfigMap(
		f *ci.Framework) (*core.ConfigMap, error) {
	cm, err := c.cmLister.ConfigMaps(f.Namespace).Get(f.ConfigMapName())
	if err != nil {
		if apiErrors.IsNotFound(err) {
			return nil, nil
		} else {
			return nil, fmt.Errorf(
				"[%v]: ConfigMap %v cannot be got from local cache: %v",
				f.Key(), f.ConfigMapName(), err)
		}
	}

	if f.ConfigMapUID() == nil || *f.ConfigMapUID() != cm.UID {
		// cm is the unmanaged
		if meta.IsControlledBy(cm, f) {
			// The managed ConfigMap becomes unmanaged if and only if Framework.Status
			// is failed to persist due to FrameworkController restart or create fails
			// but succeeds on remote, so clean up the ConfigMap to avoid unmanaged cm
			// leak.
			return nil, c.deleteConfigMap(f, cm.UID)
		} else {
			return nil, fmt.Errorf(
				"[%v]: ConfigMap naming conflicted with others, it is not controlled "+
						"by %v, consult namespace admin to resolve ConfigMap naming "+
						"conflict: %v, %v",
				f.Key(), ci.ControllerName, cm.Name, cm.UID)
		}
	} else {
		// cm is the managed
		return cm, nil
	}
}

// Using UID to ensure we delete the right object.
func (c *FrameworkController) deleteConfigMap(
		f *ci.Framework, cmUID types.UID) error {
	cmName := f.ConfigMapName()
	err := c.kClient.CoreV1().ConfigMaps(f.Namespace).Delete(cmName,
		&meta.DeleteOptions{Preconditions: &meta.Preconditions{UID: &cmUID}})
	if err != nil && !apiErrors.IsNotFound(err) {
		return fmt.Errorf("[%v]: Failed to delete ConfigMap %v, %v: %v",
			f.Key(), cmName, cmUID, err)
	} else {
		log.Infof("[%v]: Succeeded to delete ConfigMap %v, %v",
			f.Key(), cmName, cmUID)
		return nil
	}
}

func (c *FrameworkController) createConfigMap(
		f *ci.Framework) (*core.ConfigMap, error) {
	cm := f.NewConfigMap()
	remoteCM, err := c.kClient.CoreV1().ConfigMaps(f.Namespace).Create(cm)
	if err != nil {
		return nil, fmt.Errorf("[%v]: Failed to create ConfigMap %v: %v",
			f.Key(), cm.Name, err)
	} else {
		log.Infof("[%v]: Succeeded to create ConfigMap %v",
			f.Key(), cm.Name)
		return remoteCM, nil
	}
}

func (c *FrameworkController) syncTaskRoleStatuses(
		f *ci.Framework, cm *core.ConfigMap) error {
	logPfx := fmt.Sprintf("[%v]: syncTaskRoleStatuses: ", f.Key())
	log.Infof(logPfx + "Started")
	defer func() { log.Infof(logPfx + "Completed") }()

	errs := []error{}
	for _, taskRoleStatus := range f.TaskRoleStatuses() {
		log.Infof("[%v][%v]: syncTaskRoleStatus", f.Key(), taskRoleStatus.Name)
		for _, taskStatus := range taskRoleStatus.TaskStatuses {
			cancel, err := c.syncTaskState(f, cm, taskRoleStatus.Name, taskStatus.Index)
			if err != nil {
				errs = append(errs, err)
			}

			if cancel {
				log.Infof(
					"[%v][%v][%v]: Cancel Framework sync",
					f.Key(), taskRoleStatus.Name, taskStatus.Index)
				return errorAgg.NewAggregate(errs)
			}

			if err != nil {
				// The Tasks in the TaskRole have the same Spec except for the PodName,
				// so in most cases, same Platform Transient Error will return.
				log.Warnf(
					"[%v][%v][%v]: Failed to sync Task, "+
							"skip to sync the Tasks behind it in the TaskRole: %v",
					f.Key(), taskRoleStatus.Name, taskStatus.Index, err)
				break
			}
		}
	}

	return errorAgg.NewAggregate(errs)
}

func (c *FrameworkController) syncTaskState(
		f *ci.Framework, cm *core.ConfigMap,
		taskRoleName string, taskIndex int32) (cancelFrameworkSync bool, err error) {
	logPfx := fmt.Sprintf("[%v][%v][%v]: syncTaskState: ",
		f.Key(), taskRoleName, taskIndex)
	log.Infof(logPfx + "Started")
	defer func() { log.Infof(logPfx + "Completed") }()

	taskRoleSpec := f.TaskRoleSpec(taskRoleName)
	taskSpec := taskRoleSpec.Task
	taskRoleStatus := f.TaskRoleStatus(taskRoleName)
	taskStatus := f.TaskStatus(taskRoleName, taskIndex)

	// Get the ground truth readonly pod
	pod, err := c.getOrCleanupPod(f, cm, taskRoleName, taskIndex)
	if err != nil {
		return false, err
	}

	// Totally reconstruct TaskState in case Framework.Status is failed to persist
	// due to FrameworkController restart.
	if pod == nil {
		if taskStatus.PodUID() == nil {
			f.TransitionTaskState(taskRoleName, taskIndex, ci.TaskAttemptCreationPending)
		} else {
			// Avoid sync with outdated object:
			// pod is remote creation requested but not found in the local cache.
			if taskStatus.State == ci.TaskAttemptCreationRequested {
				if c.enqueueTaskAttemptCreationTimeoutCheck(f, taskRoleName, taskIndex, true) {
					log.Infof(logPfx +
							"Waiting Pod to appear in the local cache or timeout")
					return false, nil
				}

				diag := fmt.Sprintf(
					"Pod does not appear in the local cache within timeout %v, "+
							"so consider it was deleted and force delete it",
					common.SecToDuration(c.cConfig.ObjectLocalCacheCreationTimeoutSec))
				log.Warnf(logPfx + diag)

				// Ensure pod is deleted in remote to avoid managed pod leak after
				// TaskCompleted.
				err := c.deletePod(f, taskRoleName, taskIndex, *taskStatus.PodUID())
				if err != nil {
					return false, err
				}

				taskStatus.AttemptStatus.CompletionStatus =
						ci.CompletionCodePodCreationTimeout.NewCompletionStatus(diag)
			}

			if taskStatus.State != ci.TaskAttemptCompleted {
				if taskStatus.AttemptStatus.CompletionStatus == nil {
					diag := fmt.Sprintf("Pod was deleted by others")
					log.Warnf(logPfx + diag)
					taskStatus.AttemptStatus.CompletionStatus =
							ci.CompletionCodePodExternalDeleted.NewCompletionStatus(diag)
				}

				taskStatus.AttemptStatus.CompletionTime = common.PtrNow()
				f.TransitionTaskState(taskRoleName, taskIndex, ci.TaskAttemptCompleted)
				log.Infof(logPfx+
						"TaskAttemptInstance %v is completed with CompletionStatus: %v",
					*taskStatus.TaskAttemptInstanceUID(),
					taskStatus.AttemptStatus.CompletionStatus)
			}
		}
	} else {
		if pod.DeletionTimestamp == nil {
			if taskStatus.State == ci.TaskAttemptDeletionPending {
				// The CompletionStatus has been persisted, so it is safe to delete the
				// pod now.
				err := c.deletePod(f, taskRoleName, taskIndex, *taskStatus.PodUID())
				if err != nil {
					return false, err
				}
				f.TransitionTaskState(taskRoleName, taskIndex, ci.TaskAttemptDeletionRequested)
			}

			// Avoid sync with outdated object:
			// pod is remote deletion requested but not deleting or deleted in the local
			// cache.
			if taskStatus.State == ci.TaskAttemptDeletionRequested {
				// The deletion requested object will never appear again with the same UID,
				// so always just wait.
				log.Infof(logPfx +
						"Waiting Pod to disappearing or disappear in the local cache")
				return false, nil
			}

			// Possibly due to the NodeController has not heard from the kubelet who
			// manages the Pod for more than node-monitor-grace-period but less than
			// pod-eviction-timeout.
			// And after pod-eviction-timeout, the Pod will be marked as deleting, but
			// it will only be automatically deleted after the kubelet comes back and
			// kills the Pod.
			if pod.Status.Phase == core.PodUnknown {
				log.Infof(logPfx+
						"Waiting Pod to be deleted or deleting or transitioned from %v",
					pod.Status.Phase)
				return false, nil
			}

			// Below Pod fields may be available even when PodPending, such as the Pod
			// has been bound to a Node, but one or more Containers has not been started.
			taskStatus.AttemptStatus.PodIP = &pod.Status.PodIP
			taskStatus.AttemptStatus.PodHostIP = &pod.Status.HostIP

			if pod.Status.Phase == core.PodPending {
				f.TransitionTaskState(taskRoleName, taskIndex, ci.TaskAttemptPreparing)
				return false, nil
			} else if pod.Status.Phase == core.PodRunning {
				f.TransitionTaskState(taskRoleName, taskIndex, ci.TaskAttemptRunning)
				return false, nil
			} else if pod.Status.Phase == core.PodSucceeded {
				diag := fmt.Sprintf("Pod succeeded")
				log.Infof(logPfx + diag)
				c.completeTaskAttempt(f, taskRoleName, taskIndex,
					ci.CompletionCodeSucceeded.NewCompletionStatus(diag))
				return false, nil
			} else if pod.Status.Phase == core.PodFailed {
				// All Container names in a Pod must be different, so we can still identify
				// a Container even after the InitContainers is merged with the AppContainers.
				allContainerStatuses := append(
					pod.Status.InitContainerStatuses,
					pod.Status.ContainerStatuses...)

				lastContainerExitCode := common.NilInt32()
				lastContainerCompletionTime := time.Time{}
				allContainerDiags := []string{}
				for _, containerStatus := range allContainerStatuses {
					terminated := containerStatus.State.Terminated
					if terminated != nil && terminated.ExitCode != 0 {
						allContainerDiags = append(allContainerDiags, fmt.Sprintf(
							"[Container %v, ExitCode: %v, Reason: %v, Message: %v]",
							containerStatus.Name, terminated.ExitCode, terminated.Reason,
							terminated.Message))

						if lastContainerExitCode == nil ||
								lastContainerCompletionTime.Before(terminated.FinishedAt.Time) {
							lastContainerExitCode = &terminated.ExitCode
							lastContainerCompletionTime = terminated.FinishedAt.Time
						}
					}
				}

				if lastContainerExitCode == nil {
					diag := fmt.Sprintf(
						"Pod failed without any non-zero container exit code, maybe " +
								"stopped by the system")
					log.Warnf(logPfx + diag)
					c.completeTaskAttempt(f, taskRoleName, taskIndex,
						ci.CompletionCodePodFailedWithoutFailedContainer.NewCompletionStatus(diag))
				} else {
					diag := fmt.Sprintf(
						"Pod failed with non-zero container exit code: %v",
						strings.Join(allContainerDiags, ", "))
					log.Infof(logPfx + diag)
					if strings.Contains(diag, string(ci.ReasonOOMKilled)) {
						c.completeTaskAttempt(f, taskRoleName, taskIndex,
							ci.CompletionCodeContainerOOMKilled.NewCompletionStatus(diag))
					} else {
						c.completeTaskAttempt(f, taskRoleName, taskIndex,
							ci.CompletionCode(*lastContainerExitCode).NewCompletionStatus(diag))
					}
				}
				return false, nil
			} else {
				return false, fmt.Errorf(logPfx+
						"Failed: Got unrecognized Pod Phase: %v", pod.Status.Phase)
			}
		} else {
			f.TransitionTaskState(taskRoleName, taskIndex, ci.TaskAttemptDeleting)
			log.Infof(logPfx + "Waiting Pod to be deleted")
			return false, nil
		}
	}
	// At this point, taskStatus.State must be in:
	// {TaskCompleted, TaskAttemptCreationPending, TaskAttemptCompleted}

	if taskStatus.State == ci.TaskAttemptCompleted {
		// attemptToRetryTask
		retryDecision := taskSpec.RetryPolicy.ShouldRetry(
			taskStatus.RetryPolicyStatus,
			taskStatus.AttemptStatus.CompletionStatus.Type,
			0, 0)

		if taskStatus.RetryPolicyStatus.RetryDelaySec == nil {
			// RetryTask is not yet scheduled, so need to be decided.
			if retryDecision.ShouldRetry {
				// scheduleToRetryTask
				log.Infof(logPfx+
						"Will retry Task with new TaskAttempt: RetryDecision: %v",
					retryDecision)

				taskStatus.RetryPolicyStatus.RetryDelaySec = &retryDecision.DelaySec
			} else {
				// completeTask
				log.Infof(logPfx+
						"Will complete Task: RetryDecision: %v",
					retryDecision)

				taskStatus.CompletionTime = common.PtrNow()
				f.TransitionTaskState(taskRoleName, taskIndex, ci.TaskCompleted)
			}
		}

		if taskStatus.RetryPolicyStatus.RetryDelaySec != nil {
			// RetryTask is already scheduled, so just need to check timeout.
			if c.enqueueTaskRetryDelayTimeoutCheck(f, taskRoleName, taskIndex, true) {
				log.Infof(logPfx + "Waiting Task to retry after delay")
				return false, nil
			}

			// retryTask
			log.Infof(logPfx + "Retry Task")
			taskStatus.RetryPolicyStatus.TotalRetriedCount++
			if retryDecision.IsAccountable {
				taskStatus.RetryPolicyStatus.AccountableRetriedCount++
			}
			taskStatus.RetryPolicyStatus.RetryDelaySec = nil
			taskStatus.AttemptStatus = f.NewTaskAttemptStatus(
				taskRoleName, taskIndex, taskStatus.RetryPolicyStatus.TotalRetriedCount)
			f.TransitionTaskState(taskRoleName, taskIndex, ci.TaskAttemptCreationPending)
		}
	}
	// At this point, taskStatus.State must be in:
	// {TaskCompleted, TaskAttemptCreationPending}

	// Totally reconstruct actions triggered by TaskCompleted in case these actions
	// are missed due to FrameworkController restart.
	if taskStatus.State == ci.TaskCompleted {
		// attemptToCompleteFrameworkAttempt
		completionPolicy := taskRoleSpec.FrameworkAttemptCompletionPolicy
		minFailedTaskCount := completionPolicy.MinFailedTaskCount
		minSucceededTaskCount := completionPolicy.MinSucceededTaskCount

		if taskStatus.IsFailed() && minFailedTaskCount != ci.UnlimitedValue {
			failedTaskCount := taskRoleStatus.GetTaskCount((*ci.TaskStatus).IsFailed)
			if failedTaskCount >= minFailedTaskCount {
				diag := fmt.Sprintf(
					"TaskRole %v: FailedTaskCount %v has reached MinFailedTaskCount %v: "+
							"Triggered by Task: [Index: %v, Diagnostics: %v]",
					taskRoleName, failedTaskCount, minFailedTaskCount,
					taskIndex, taskStatus.AttemptStatus.CompletionStatus.Diagnostics)
				log.Infof(logPfx + diag)
				c.completeFrameworkAttempt(f,
					taskStatus.AttemptStatus.CompletionStatus.Code.NewCompletionStatus(diag))
				return true, nil
			}
		}

		if taskStatus.IsSucceeded() && minSucceededTaskCount != ci.UnlimitedValue {
			succeededTaskCount := taskRoleStatus.GetTaskCount((*ci.TaskStatus).IsSucceeded)
			if succeededTaskCount >= minSucceededTaskCount {
				diag := fmt.Sprintf(
					"TaskRole %v: SucceededTaskCount %v has reached MinSucceededTaskCount %v: "+
							"Triggered by Task: [Index: %v, Diagnostics: %v]",
					taskRoleName, succeededTaskCount, minSucceededTaskCount,
					taskIndex, taskStatus.AttemptStatus.CompletionStatus.Diagnostics)
				log.Infof(logPfx + diag)
				c.completeFrameworkAttempt(f,
					ci.CompletionCodeSucceeded.NewCompletionStatus(diag))
				return true, nil
			}
		}

		if f.AreAllTasksCompleted() {
			totalTaskCount := f.GetTaskCount(nil)
			failedTaskCount := f.GetTaskCount((*ci.TaskStatus).IsFailed)
			diag := fmt.Sprintf(
				"All Tasks are completed and no FrameworkAttemptCompletionPolicy has "+
						"ever been triggered: TotalTaskCount: %v, FailedTaskCount: %v: "+
						"Triggered by Task: [Index: %v, Diagnostics: %v]",
				totalTaskCount, failedTaskCount,
				taskIndex, taskStatus.AttemptStatus.CompletionStatus.Diagnostics)
			log.Infof(logPfx + diag)
			c.completeFrameworkAttempt(f,
				ci.CompletionCodeSucceeded.NewCompletionStatus(diag))
			return true, nil
		}

		return false, nil
	}
	// At this point, taskStatus.State must be in:
	// {TaskAttemptCreationPending}

	if taskStatus.State == ci.TaskAttemptCreationPending {
		// createTaskAttempt
		pod, err := c.createPod(f, cm, taskRoleName, taskIndex)
		if err != nil {
			apiErr := errorWrap.Cause(err)
			if apiErrors.IsInvalid(apiErr) {
				// Should be Framework Error instead of Platform Transient Error.
				// Directly complete the FrameworkAttempt, since we should not complete
				// a TaskAttempt without an associated Pod in any case.
				diag := fmt.Sprintf(
					"TaskRole %v: Pod Spec is invalid: %v", taskRoleName, apiErr)
				log.Infof(logPfx + diag)
				c.completeFrameworkAttempt(f,
					ci.CompletionCodePodSpecInvalid.NewCompletionStatus(diag))
				return true, nil
			} else {
				return false, err
			}
		}

		taskStatus.AttemptStatus.PodUID = &pod.UID
		taskStatus.AttemptStatus.InstanceUID = ci.GetTaskAttemptInstanceUID(
			taskStatus.TaskAttemptID(), taskStatus.PodUID())
		f.TransitionTaskState(taskRoleName, taskIndex, ci.TaskAttemptCreationRequested)

		// Informer may not deliver any event if a create is immediately followed by
		// a delete, so manually enqueue a sync to check the pod existence after the
		// timeout.
		c.enqueueTaskAttemptCreationTimeoutCheck(f, taskRoleName, taskIndex, false)

		// The ground truth pod is the local cached one instead of the remote one,
		// so need to wait before continue the sync.
		log.Infof(logPfx +
				"Waiting Pod to appear in the local cache or timeout")
		return false, nil
	}
	// At this point, taskStatus.State must be in:
	// {}

	// Unreachable
	panic(fmt.Errorf(logPfx+
			"Failed: At this point, TaskState should be in {} instead of %v",
		taskStatus.State))
}

// Get Task's current Pod object, if not found, then clean up existing
// controlled Pod if any.
// Returned pod is either managed or nil, if it is the managed pod, it is not
// writable and may be outdated even if no error.
// Clean up instead of recovery is because the PodUID is always the ground truth.
func (c *FrameworkController) getOrCleanupPod(
		f *ci.Framework, cm *core.ConfigMap,
		taskRoleName string, taskIndex int32) (*core.Pod, error) {
	taskStatus := f.TaskStatus(taskRoleName, taskIndex)
	pod, err := c.podLister.Pods(f.Namespace).Get(taskStatus.PodName())
	if err != nil {
		if apiErrors.IsNotFound(err) {
			return nil, nil
		} else {
			return nil, fmt.Errorf(
				"[%v][%v][%v]: Pod %v cannot be got from local cache: %v",
				f.Key(), taskRoleName, taskIndex, taskStatus.PodName(), err)
		}
	}

	if taskStatus.PodUID() == nil || *taskStatus.PodUID() != pod.UID {
		// pod is the unmanaged
		if meta.IsControlledBy(pod, cm) {
			// The managed Pod becomes unmanaged if and only if Framework.Status
			// is failed to persist due to FrameworkController restart or create fails
			// but succeeds on remote, so clean up the Pod to avoid unmanaged pod leak.
			return nil, c.deletePod(f, taskRoleName, taskIndex, pod.UID)
		} else {
			return nil, fmt.Errorf(
				"[%v][%v][%v]: Pod naming conflicted with others, it is not controlled "+
						"by %v, consult namespace admin to resolve Pod naming "+
						"conflict: %v, %v",
				f.Key(), taskRoleName, taskIndex, ci.ControllerName, pod.Name, pod.UID)
		}
	} else {
		// pod is the managed
		return pod, nil
	}
}

// Using UID to ensure we delete the right object.
func (c *FrameworkController) deletePod(
		f *ci.Framework, taskRoleName string, taskIndex int32,
		podUID types.UID) error {
	taskStatus := f.TaskStatus(taskRoleName, taskIndex)
	podName := taskStatus.PodName()
	err := c.kClient.CoreV1().Pods(f.Namespace).Delete(podName,
		&meta.DeleteOptions{Preconditions: &meta.Preconditions{UID: &podUID}})
	if err != nil && !apiErrors.IsNotFound(err) {
		return fmt.Errorf("[%v][%v][%v]: Failed to delete Pod %v, %v: %v",
			f.Key(), taskRoleName, taskIndex, podName, podUID, err)
	} else {
		log.Infof("[%v][%v][%v]: Succeeded to delete Pod %v, %v",
			f.Key(), taskRoleName, taskIndex, podName, podUID)
		return nil
	}
}

func (c *FrameworkController) createPod(
		f *ci.Framework, cm *core.ConfigMap,
		taskRoleName string, taskIndex int32) (*core.Pod, error) {
	pod := f.NewPod(cm, taskRoleName, taskIndex)
	remotePod, err := c.kClient.CoreV1().Pods(f.Namespace).Create(pod)
	if err != nil {
		return nil, errorWrap.Wrapf(err,
			"[%v]: Failed to create Pod %v", f.Key(), pod.Name)
	} else {
		log.Infof("[%v]: Succeeded to create Pod: %v", f.Key(), pod.Name)
		return remotePod, nil
	}
}

func (c *FrameworkController) completeTaskAttempt(
		f *ci.Framework, taskRoleName string, taskIndex int32,
		completionStatus *ci.CompletionStatus) {
	logPfx := fmt.Sprintf("[%v][%v][%v]: completeTaskAttempt: ",
		f.Key(), taskRoleName, taskIndex)

	taskStatus := f.TaskStatus(taskRoleName, taskIndex)
	taskStatus.AttemptStatus.CompletionStatus = completionStatus
	f.TransitionTaskState(taskRoleName, taskIndex, ci.TaskAttemptDeletionPending)

	// To ensure the CompletionStatus is persisted before deleting the pod,
	// we need to wait until next sync to delete the pod, so manually enqueue
	// a sync.
	c.enqueueFramework(f, "TaskAttemptDeletionPending")
	log.Infof(logPfx + "Waiting TaskAttempt CompletionStatus to be persisted")
}

func (c *FrameworkController) completeFrameworkAttempt(
		f *ci.Framework, completionStatus *ci.CompletionStatus) {
	logPfx := fmt.Sprintf("[%v]: completeFrameworkAttempt: ", f.Key())

	f.Status.AttemptStatus.CompletionStatus = completionStatus
	f.TransitionFrameworkState(ci.FrameworkAttemptDeletionPending)

	// To ensure the CompletionStatus is persisted before deleting the cm,
	// we need to wait until next sync to delete the cm, so manually enqueue
	// a sync.
	c.enqueueFramework(f, "FrameworkAttemptDeletionPending")
	log.Infof(logPfx + "Waiting FrameworkAttempt CompletionStatus to be persisted")
}

func (c *FrameworkController) updateRemoteFrameworkStatus(f *ci.Framework) error {
	logPfx := fmt.Sprintf("[%v]: updateRemoteFrameworkStatus: ", f.Key())
	log.Infof(logPfx + "Started")
	defer func() { log.Infof(logPfx + "Completed") }()

	updateF := f
	updateErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		updateF.Status = f.Status
		_, updateErr := c.fClient.FrameworkcontrollerV1().Frameworks(updateF.Namespace).Update(updateF)
		if updateErr == nil {
			return nil
		}

		// Try to resolve conflict by patching more recent object.
		localF, getErr := c.fLister.Frameworks(updateF.Namespace).Get(updateF.Name)
		if getErr != nil {
			if apiErrors.IsNotFound(getErr) {
				return fmt.Errorf("Framework cannot be found in local cache: %v", getErr)
			} else {
				log.Warnf(logPfx+"Framework cannot be got from local cache: %v", getErr)
			}
		} else {
			// Only resolve conflict for the same object to avoid updating another
			// object of the same name.
			if f.UID != localF.UID {
				return fmt.Errorf(
					"Framework UID mismatch: Current UID %v, Local Cached UID %v",
					f.UID, localF.UID)
			} else {
				updateF = localF.DeepCopy()
			}
		}

		return updateErr
	})

	if updateErr != nil {
		// Will still be requeued and retried after rate limited delay.
		return fmt.Errorf(logPfx+"Failed: %v", updateErr)
	} else {
		return nil
	}
}

func (c *FrameworkController) getExpectedFrameworkStatusInfo(key string) (
		*ExpectedFrameworkStatusInfo, bool) {
	value, exists := c.fExpectedStatusInfos[key]
	return value, exists
}

func (c *FrameworkController) deleteExpectedFrameworkStatusInfo(key string) {
	log.Infof("[%v]: deleteExpectedFrameworkStatusInfo: ", key)
	delete(c.fExpectedStatusInfos, key)
}

func (c *FrameworkController) updateExpectedFrameworkStatusInfo(key string,
		status *ci.FrameworkStatus, remoteSynced bool) {
	log.Infof("[%v]: updateExpectedFrameworkStatusInfo", key)
	c.fExpectedStatusInfos[key] = &ExpectedFrameworkStatusInfo{
		status:       status,
		remoteSynced: remoteSynced,
	}
}
