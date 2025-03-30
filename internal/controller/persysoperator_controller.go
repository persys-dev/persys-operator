/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	podv1alpha1 "github.com/persys-dev/persys-operator/api/v1alpha1"
)

// PersysOperatorReconciler reconciles a PersysOperator object
type PersysOperatorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=prow.persys.io,resources=persysoperators,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=prow.persys.io,resources=persysoperators/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete

func (r *PersysOperatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the PersysOperator instance
	persysOperator := &podv1alpha1.PersysOperator{}
	err := r.Get(ctx, req.NamespacedName, persysOperator)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch PersysOperator")
		return ctrl.Result{}, err
	}

	// List existing pods with matching labels
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(req.Namespace),
		client.MatchingLabels{"managed-by": req.Name},
	}
	if err = r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "failed to list pods")
		return ctrl.Result{}, err
	}

	currentPods := len(podList.Items)
	desiredPods := int(persysOperator.Spec.Replicas)

	// Create pods if needed
	for i := currentPods; i < desiredPods; i++ {
		pod := r.createPod(persysOperator, i)
		if err := r.Create(ctx, pod); err != nil {
			log.Error(err, "failed to create pod")
			return ctrl.Result{}, err
		}
	}

	// Delete excess pods
	for i, pod := range podList.Items {
		if i >= desiredPods {
			if err := r.Delete(ctx, &pod); err != nil {
				log.Error(err, "failed to delete pod")
				return ctrl.Result{}, err
			}
		}
	}

	// Update status
	podNames := make([]string, 0, len(podList.Items))
	for _, pod := range podList.Items {
		podNames = append(podNames, pod.Name)
	}
	persysOperator.Status.CurrentReplicas = int32(currentPods)
	persysOperator.Status.PodNames = podNames
	if err := r.Status().Update(ctx, persysOperator); err != nil {
		log.Error(err, "failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *PersysOperatorReconciler) createPod(po *podv1alpha1.PersysOperator, index int) *corev1.Pod {
	podName := fmt.Sprintf("%s-%d", po.Name, index)
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: po.Namespace,
			Labels:    map[string]string{"managed-by": po.Name},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(po, podv1alpha1.GroupVersion.WithKind("PersysOperator")),
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  podName,
					Image: po.Spec.Image,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse(po.Spec.CPU),
							corev1.ResourceMemory: resource.MustParse(po.Spec.Memory),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse(po.Spec.CPU),
							corev1.ResourceMemory: resource.MustParse(po.Spec.Memory),
						},
					},
				},
			},
		},
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *PersysOperatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&podv1alpha1.PersysOperator{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}
