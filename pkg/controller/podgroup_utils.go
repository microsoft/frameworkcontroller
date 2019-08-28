package controller

import (
	"fmt"
	"github.com/kubernetes-sigs/kube-batch/pkg/apis/scheduling/v1alpha1"
	v1 "github.com/microsoft/frameworkcontroller/pkg/apis/frameworkcontroller/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog"
)
// todo frameworkController add podgroupInformer and save to cache
func (c *FrameworkController) SyncPodGroup(f *v1.Framework, minAvailableReplicas int32) (*v1alpha1.PodGroup, error) {

	kubeBatchClientInterface := c.kbClient
	// Check whether podGroup exists or not
	podGroupName := GenPodGroupName(f.Name)
	podGroup, err := kubeBatchClientInterface.SchedulingV1alpha1().PodGroups(f.Namespace).Get(podGroupName, metav1.GetOptions{})
	if err == nil {
		return podGroup, nil
	}

	// create podGroup with
	minAvailable := intstr.FromInt(int(minAvailableReplicas))
	priorityClassName := f.Spec.PriorityClassName
	queueName := f.Spec.Queue
	klog.Infof("create podgroup [%s/%s] minNumber %v, PriorityClassName %v, Queue %v", f.Namespace, podGroupName, priorityClassName, queueName)
	createPodGroup := &v1alpha1.PodGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podGroupName,
			Namespace: f.Namespace,
		},
		Spec: v1alpha1.PodGroupSpec{
			MinMember:         minAvailable.IntVal,
			PriorityClassName: priorityClassName,
			Queue:             queueName,
		},
	}
	return kubeBatchClientInterface.SchedulingV1alpha1().PodGroups(f.Namespace).Create(createPodGroup)
}
func (jc *FrameworkController) DeletePodGroup(object runtime.Object) error {
	kubeBatchClientInterface := jc.kbClient

	accessor, err := meta.Accessor(object)
	if err != nil {
		return fmt.Errorf("object does not have ObjectMeta, %v", err)
	}

	//check whether podGroup exists or not
	_, err = kubeBatchClientInterface.SchedulingV1alpha1().PodGroups(accessor.GetNamespace()).Get(accessor.GetName(), metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	klog.Infof("Deleting PodGroup %s, namespace %s,accessor %v", accessor.GetName(), accessor.GetNamespace(), accessor)

	//delete podGroup
	err = kubeBatchClientInterface.SchedulingV1alpha1().PodGroups(accessor.GetNamespace()).Delete(accessor.GetName(), &metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("unable to delete PodGroup: %v", err)
	}
	klog.Infof("Deleted PodGroup %s", accessor.GetName())
	return nil
}

// Gen PodGroupName for kube-batch, which is used for crd podGroup and annotation in pod
func GenPodGroupName(jobName string) string {
	return jobName
}
