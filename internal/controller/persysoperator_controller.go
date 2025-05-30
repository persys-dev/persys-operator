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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	workloadv1alpha1 "github.com/persys-dev/persys-operator/api/v1alpha1"
)

// PersysWorkloadReconciler reconciles a PersysWorkload object
type PersysWorkloadReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=prow.persys.io,resources=persysworkloads,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=prow.persys.io,resources=persysworkloads/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;update

func (r *PersysWorkloadReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch PersysWorkload
	workload := &workloadv1alpha1.PersysWorkload{}
	err := r.Get(ctx, req.NamespacedName, workload)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch PersysWorkload")
		return ctrl.Result{}, err
	}

	// Check for webhook updates
	if err := r.checkWebhookUpdates(ctx, workload); err != nil {
		log.Error(err, "failed to process webhook updates")
	}

	// Handle deletion
	if !workload.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.handleDeletion(ctx, workload)
	}

	// Reconcile based on workloadType
	switch workload.Spec.WorkloadType {
	case "Deployment":
		return r.reconcileDeployment(ctx, workload)
	case "StatefulSet":
		return r.reconcileStatefulSet(ctx, workload)
	case "Job":
		return r.reconcileJob(ctx, workload)
	case "CronJob":
		return r.reconcileCronJob(ctx, workload)
	default:
		log.Error(nil, "unsupported workload type", "type", workload.Spec.WorkloadType)
		return ctrl.Result{}, fmt.Errorf("unsupported workload type: %s", workload.Spec.WorkloadType)
	}
}

func (r *PersysWorkloadReconciler) checkWebhookUpdates(ctx context.Context, workload *workloadv1alpha1.PersysWorkload) error {
	cm := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Name: "persys-webhooks", Namespace: workload.Namespace}, cm); err != nil {
		return nil // ConfigMap not found, skip
	}
	for i, container := range workload.Spec.Containers {
		if newImage, ok := cm.Data[workload.Name+"-"+container.Name]; ok && newImage != container.Image {
			workload.Spec.Containers[i].Image = newImage
			if err := r.Update(ctx, workload); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *PersysWorkloadReconciler) handleDeletion(ctx context.Context, workload *workloadv1alpha1.PersysWorkload) error {
	switch workload.Spec.WorkloadType {
	case "Deployment":
		dep := &appsv1.Deployment{}
		if err := r.Get(ctx, types.NamespacedName{Name: workload.Name, Namespace: workload.Namespace}, dep); err == nil {
			return r.Delete(ctx, dep)
		}
	case "StatefulSet":
		ss := &appsv1.StatefulSet{}
		if err := r.Get(ctx, types.NamespacedName{Name: workload.Name, Namespace: workload.Namespace}, ss); err == nil {
			return r.Delete(ctx, ss)
		}
	case "Job":
		job := &batchv1.Job{}
		if err := r.Get(ctx, types.NamespacedName{Name: workload.Name, Namespace: workload.Namespace}, job); err == nil {
			return r.Delete(ctx, job)
		}
	case "CronJob":
		cronJob := &batchv1.CronJob{}
		if err := r.Get(ctx, types.NamespacedName{Name: workload.Name, Namespace: workload.Namespace}, cronJob); err == nil {
			return r.Delete(ctx, cronJob)
		}
	}
	return nil
}

func (r *PersysWorkloadReconciler) reconcileDeployment(ctx context.Context, workload *workloadv1alpha1.PersysWorkload) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	dep, err := r.createOrUpdateDeployment(ctx, workload)
	if err != nil {
		log.Error(err, "failed to manage Deployment")
		return ctrl.Result{}, err
	}

	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(workload.Namespace),
		client.MatchingLabels{"app": workload.Name},
	}
	if err := r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "failed to list pods")
		return ctrl.Result{}, err
	}

	state := "Pending"
	if dep.Status.AvailableReplicas >= workload.Spec.Replicas {
		state = "Running"
	} else {
		for _, pod := range podList.Items {
			if pod.Status.Phase == corev1.PodFailed {
				state = "Failed"
				break
			}
		}
	}

	return r.updateStatus(ctx, workload, state, dep.Status.AvailableReplicas, podList)
}

func (r *PersysWorkloadReconciler) createOrUpdateDeployment(ctx context.Context, workload *workloadv1alpha1.PersysWorkload) (*appsv1.Deployment, error) {
	depName := types.NamespacedName{Name: workload.Name, Namespace: workload.Namespace}
	dep := &appsv1.Deployment{}
	err := r.Get(ctx, depName, dep)
	create := errors.IsNotFound(err)
	if err != nil && !create {
		return nil, err
	}

	desiredDep := r.buildDeployment(workload)
	if create {
		if err := r.Create(ctx, desiredDep); err != nil {
			return nil, err
		}
		return desiredDep, nil
	}

	dep.Spec = desiredDep.Spec
	if err := r.Update(ctx, dep); err != nil {
		return nil, err
	}
	return dep, nil
}

func (r *PersysWorkloadReconciler) buildDeployment(workload *workloadv1alpha1.PersysWorkload) *appsv1.Deployment {
	labels := map[string]string{"app": workload.Name}
	containers := make([]corev1.Container, len(workload.Spec.Containers))
	for i, c := range workload.Spec.Containers {
		containers[i] = corev1.Container{
			Name:         c.Name,
			Image:        c.Image,
			Command:      c.Command,
			Env:          c.Env,
			Ports:        c.Ports,
			VolumeMounts: c.VolumeMounts,
			Resources:    c.Resources,
		}
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workload.Name,
			Namespace: workload.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(workload, workloadv1alpha1.GroupVersion.WithKind("PersysWorkload")),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &workload.Spec.Replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					NodeSelector:  workload.Spec.NodeSelector,
					Affinity:      workload.Spec.Affinity,
					Tolerations:   workload.Spec.Tolerations,
					Containers:    containers,
					RestartPolicy: workload.Spec.RestartPolicy,
				},
			},
		},
	}
}

func (r *PersysWorkloadReconciler) reconcileStatefulSet(ctx context.Context, workload *workloadv1alpha1.PersysWorkload) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	ss, err := r.createOrUpdateStatefulSet(ctx, workload)
	if err != nil {
		log.Error(err, "failed to manage StatefulSet")
		return ctrl.Result{}, err
	}

	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(workload.Namespace),
		client.MatchingLabels{"app": workload.Name},
	}
	if err := r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "failed to list pods")
		return ctrl.Result{}, err
	}

	state := "Pending"
	if ss.Status.AvailableReplicas >= workload.Spec.Replicas {
		state = "Running"
	} else {
		for _, pod := range podList.Items {
			if pod.Status.Phase == corev1.PodFailed {
				state = "Failed"
				break
			}
		}
	}

	return r.updateStatus(ctx, workload, state, ss.Status.AvailableReplicas, podList)
}

func (r *PersysWorkloadReconciler) createOrUpdateStatefulSet(ctx context.Context, workload *workloadv1alpha1.PersysWorkload) (*appsv1.StatefulSet, error) {
	ssName := types.NamespacedName{Name: workload.Name, Namespace: workload.Namespace}
	ss := &appsv1.StatefulSet{}
	err := r.Get(ctx, ssName, ss)
	create := errors.IsNotFound(err)
	if err != nil && !create {
		return nil, err
	}

	desiredSS := r.buildStatefulSet(workload)
	if create {
		if err := r.Create(ctx, desiredSS); err != nil {
			return nil, err
		}
		return desiredSS, nil
	}

	ss.Spec = desiredSS.Spec
	if err := r.Update(ctx, ss); err != nil {
		return nil, err
	}
	return ss, nil
}

func (r *PersysWorkloadReconciler) buildStatefulSet(workload *workloadv1alpha1.PersysWorkload) *appsv1.StatefulSet {
	labels := map[string]string{"app": workload.Name}
	containers := make([]corev1.Container, len(workload.Spec.Containers))
	for i, c := range workload.Spec.Containers {
		containers[i] = corev1.Container{
			Name:         c.Name,
			Image:        c.Image,
			Command:      c.Command,
			Env:          c.Env,
			Ports:        c.Ports,
			VolumeMounts: c.VolumeMounts,
			Resources:    c.Resources,
		}
	}

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workload.Name,
			Namespace: workload.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(workload, workloadv1alpha1.GroupVersion.WithKind("PersysWorkload")),
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &workload.Spec.Replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					NodeSelector:  workload.Spec.NodeSelector,
					Affinity:      workload.Spec.Affinity,
					Tolerations:   workload.Spec.Tolerations,
					Containers:    containers,
					RestartPolicy: workload.Spec.RestartPolicy,
				},
			},
		},
	}
}

func (r *PersysWorkloadReconciler) reconcileJob(ctx context.Context, workload *workloadv1alpha1.PersysWorkload) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	job, err := r.createOrUpdateJob(ctx, workload)
	if err != nil {
		log.Error(err, "failed to manage Job")
		return ctrl.Result{}, err
	}

	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(workload.Namespace),
		client.MatchingLabels{"app": workload.Name},
	}
	if err := r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "failed to list pods")
		return ctrl.Result{}, err
	}

	state := "Pending"
	if job.Status.Succeeded > 0 {
		state = "Completed"
	} else if job.Status.Failed > 0 {
		state = "Failed"
	}

	return r.updateStatus(ctx, workload, state, 1, podList)
}

func (r *PersysWorkloadReconciler) createOrUpdateJob(ctx context.Context, workload *workloadv1alpha1.PersysWorkload) (*batchv1.Job, error) {
	jobName := types.NamespacedName{Name: workload.Name, Namespace: workload.Namespace}
	job := &batchv1.Job{}
	err := r.Get(ctx, jobName, job)
	create := errors.IsNotFound(err)
	if err != nil && !create {
		return nil, err
	}

	desiredJob := r.buildJob(workload)
	if create {
		if err := r.Create(ctx, desiredJob); err != nil {
			return nil, err
		}
		return desiredJob, nil
	}

	job.Spec = desiredJob.Spec
	if err := r.Update(ctx, job); err != nil {
		return nil, err
	}
	return job, nil
}

func (r *PersysWorkloadReconciler) buildJob(workload *workloadv1alpha1.PersysWorkload) *batchv1.Job {
	labels := map[string]string{"app": workload.Name}
	containers := make([]corev1.Container, len(workload.Spec.Containers))
	for i, c := range workload.Spec.Containers {
		containers[i] = corev1.Container{
			Name:         c.Name,
			Image:        c.Image,
			Command:      c.Command,
			Env:          c.Env,
			Ports:        c.Ports,
			VolumeMounts: c.VolumeMounts,
			Resources:    c.Resources,
		}
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workload.Name,
			Namespace: workload.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(workload, workloadv1alpha1.GroupVersion.WithKind("PersysWorkload")),
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					NodeSelector:  workload.Spec.NodeSelector,
					Affinity:      workload.Spec.Affinity,
					Tolerations:   workload.Spec.Tolerations,
					Containers:    containers,
					RestartPolicy: workload.Spec.RestartPolicy,
				},
			},
		},
	}
}

func (r *PersysWorkloadReconciler) reconcileCronJob(ctx context.Context, workload *workloadv1alpha1.PersysWorkload) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	cronJob, err := r.createOrUpdateCronJob(ctx, workload)
	if err != nil {
		log.Error(err, "failed to manage CronJob")
		return ctrl.Result{}, err
	}

	if cronJob != nil {
		log.Info("cronjob not empty")
	}

	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(workload.Namespace),
		client.MatchingLabels{"app": workload.Name},
	}
	if err := r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "failed to list pods")
		return ctrl.Result{}, err
	}

	state := "Running"
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodFailed {
			state = "Failed"
			break
		}
	}

	return r.updateStatus(ctx, workload, state, 1, podList)
}

func (r *PersysWorkloadReconciler) createOrUpdateCronJob(ctx context.Context, workload *workloadv1alpha1.PersysWorkload) (*batchv1.CronJob, error) {
	cronJobName := types.NamespacedName{Name: workload.Name, Namespace: workload.Namespace}
	cronJob := &batchv1.CronJob{}
	err := r.Get(ctx, cronJobName, cronJob)
	create := errors.IsNotFound(err)
	if err != nil && !create {
		return nil, err
	}

	desiredCronJob := r.buildCronJob(workload)
	if create {
		if err := r.Create(ctx, desiredCronJob); err != nil {
			return nil, err
		}
		return desiredCronJob, nil
	}

	cronJob.Spec = desiredCronJob.Spec
	if err := r.Update(ctx, cronJob); err != nil {
		return nil, err
	}
	return cronJob, nil
}

func (r *PersysWorkloadReconciler) buildCronJob(workload *workloadv1alpha1.PersysWorkload) *batchv1.CronJob {
	labels := map[string]string{"app": workload.Name}
	containers := make([]corev1.Container, len(workload.Spec.Containers))
	for i, c := range workload.Spec.Containers {
		containers[i] = corev1.Container{
			Name:         c.Name,
			Image:        c.Image,
			Command:      c.Command,
			Env:          c.Env,
			Ports:        c.Ports,
			VolumeMounts: c.VolumeMounts,
			Resources:    c.Resources,
		}
	}

	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workload.Name,
			Namespace: workload.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(workload, workloadv1alpha1.GroupVersion.WithKind("PersysWorkload")),
			},
		},
		Spec: batchv1.CronJobSpec{
			Schedule: workload.Spec.CronSchedule,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: labels},
						Spec: corev1.PodSpec{
							NodeSelector:  workload.Spec.NodeSelector,
							Affinity:      workload.Spec.Affinity,
							Tolerations:   workload.Spec.Tolerations,
							Containers:    containers,
							RestartPolicy: workload.Spec.RestartPolicy,
						},
					},
				},
			},
		},
	}
}

func (r *PersysWorkloadReconciler) updateStatus(ctx context.Context, workload *workloadv1alpha1.PersysWorkload, state string, replicas int32, podList *corev1.PodList) (ctrl.Result, error) {
	podNames := make([]string, 0, len(podList.Items))
	for _, pod := range podList.Items {
		podNames = append(podNames, pod.Name)
	}
	workload.Status.State = state
	workload.Status.CurrentReplicas = replicas
	workload.Status.PodNames = podNames
	workload.Status.LastUpdateTime = &metav1.Time{Time: time.Now()}
	if err := r.Status().Update(ctx, workload); err != nil {
		log.FromContext(ctx).Error(err, "failed to update status")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *PersysWorkloadReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&workloadv1alpha1.PersysWorkload{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}
