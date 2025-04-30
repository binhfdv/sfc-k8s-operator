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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkingv1alpha1 "github.com/binhfdv/sfc-k8s-operator/api/v1alpha1"
)

// ServiceFunctionReconciler reconciles a ServiceFunction object
type ServiceFunctionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=networking.sfc.comnets.com,resources=servicefunctions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.sfc.comnets.com,resources=servicefunctions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.sfc.comnets.com,resources=servicefunctions/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ServiceFunction object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.4/pkg/reconcile
func (r *ServiceFunctionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx).WithValues("servicefunction", req.NamespacedName)
	log.Info("Reconcile called")

	// Fetch the ServiceFunction instance
	instance := &networkingv1alpha1.ServiceFunction{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		log.Error(err, "unable to fetch ServiceFunction")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log.Info("\n=========================================================\n" +
		"-------------- Service Function Controller --------------\n" +
		"=========================================================\n")

	forwarderPod, err := r.createOrUpdateForwarderPod(ctx, instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	hostSFPod, err := r.createOrUpdateHostSFPod(ctx, instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Check readiness
	forwarderReady := isPodReady(forwarderPod)
	hostSFReady := isPodReady(hostSFPod)
	log.Info("Forwarder Pod Ready: ", "status", forwarderReady)
	log.Info("Host Service Function Pod Ready: ", "status", hostSFReady)

	// fmt.Println("======== pod status: ", foundPod.Status)
	// fmt.Println("======== pod phase: ", foundPod.Status.Phase)
	// fmt.Println("======== pod conditions: ", foundPod.Status.Conditions)
	// fmt.Println("======== pod: ", foundPod.Labels)

	// if foundPod.Status.Phase == corev1.PodPending {
	// 	log.Info("Pod is still pending. Waiting for containers to start...")
	// 	return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
	// }

	// if foundPod.Status.Phase == corev1.PodFailed || foundPod.Status.Phase == corev1.PodUnknown {
	// 	log.Error(nil, "Service Function Pod failed or unknown, recreating", "status", foundPod.Status.Phase)

	// 	err := r.Delete(ctx, foundPod)
	// 	if err != nil {
	// 		log.Error(err, "failed to delete unhealthy Service Function pod")
	// 		return ctrl.Result{}, err
	// 	}

	// 	return ctrl.Result{Requeue: true}, nil
	// }

	updatedStatus := instance.Status.DeepCopy()
	updatedStatus.LastUpdated = metav1.Now()

	if forwarderReady && hostSFReady {
		log.Info("Both forwarder and host service function pods are ready")
		updatedStatus.Ready = true
		updatedStatus.PodName = forwarderPod.Name // Optional: track both names if needed
	} else {
		log.Info("Waiting for both pods to be ready")
		updatedStatus.Ready = false
	}

	if instance.Status.Ready != updatedStatus.Ready ||
		instance.Status.PodName != updatedStatus.PodName ||
		!instance.Status.LastUpdated.Equal(&updatedStatus.LastUpdated) {

		instance.Status = *updatedStatus
		if err := r.Status().Update(ctx, instance); err != nil {
			log.Error(err, "unable to update ServiceFunction status")
			return ctrl.Result{}, err
		}
	}

	// return ctrl.Result{}, nil
	return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceFunctionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.ServiceFunction{}).
		Owns(&corev1.Pod{}).
		Named("servicefunction").
		Complete(r)
}

func (r *ServiceFunctionReconciler) createOrUpdateForwarderPod(ctx context.Context, instance *networkingv1alpha1.ServiceFunction) (*corev1.Pod, error) {
	log := logf.FromContext(ctx).WithValues("servicefunction", instance.Namespace)

	var clusterIP string
	var targetPort int32

	// Get all SFCs and use the first one
	sfcList := &networkingv1alpha1.ServiceFunctionChainList{}
	if err := r.List(ctx, sfcList); err != nil {
		log.Error(err, "failed to list ServiceFunctionChains")
		return nil, err
	}

	var sfList []string
	if len(sfcList.Items) > 0 {
		sfList = sfcList.Items[0].Spec.Functions
	}

	// Decide the forwarding target
	if len(sfList) == 0 && instance.Spec.NextSF.Name == "" {
		return nil, fmt.Errorf("no service functions defined in the chain and no next service function specified")
	} else if instance.Spec.NextSF.Name != "" {
		// Explicit nextSF provided
		targetHost := instance.Spec.NextSF.Name
		var err error
		clusterIP, targetPort, err = r.getFunctionServiceInfo(ctx, instance.Namespace, targetHost)
		if err != nil {
			log.Error(err, "failed to get next Service Function's service info")
			return nil, err
		}
	} else {
		// Use the next function from the SFC list
		sfIndexMap := make(map[string]int)
		for i, sf := range sfList {
			sfIndexMap[sf] = i
		}

		if index, ok := sfIndexMap[instance.Name]; ok && index+1 < len(sfList) {
			targetHost := sfList[index+1]
			var err error
			clusterIP, targetPort, err = r.getFunctionServiceInfo(ctx, instance.Namespace, targetHost)
			if err != nil {
				log.Error(err, "failed to get next Service Function's service info")
				return nil, err
			}
		} else {
			log.Info("Current service function is last in chain or not found")
			// Optionally, return nil here or let the forwarder do nothing
			clusterIP = "127.0.0.1"
			targetPort = 80
		}
	}

	// Convert port to string
	portStr := fmt.Sprintf("%d", targetPort)

	// Create pod spec
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			Labels:    instance.Labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  instance.Name,
					Image: instance.Spec.Image,
					Ports: instance.Spec.Ports,
					Env: []corev1.EnvVar{
						{
							Name:  "TARGET_HOST",
							Value: clusterIP,
						},
						{
							Name:  "TARGET_PORT",
							Value: portStr,
						},
					},
				},
			},
			NodeSelector: instance.Spec.NodeSelector,
		},
	}

	if err := controllerutil.SetControllerReference(instance, pod, r.Scheme); err != nil {
		log.Error(err, "unable to set controller reference on pod")
		return nil, err
	}

	foundPod := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, foundPod)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating Forwarder pod", "pod", pod.Name)
		if err := r.Create(ctx, pod); err != nil {
			log.Error(err, "unable to create Forwarder pod", "pod", pod.Name)
			return nil, err
		}
		return pod, nil
	} else if err != nil {
		log.Error(err, "unable to fetch Forwarder pod", "pod", pod.Name)
		return nil, err
	}

	log.Info("Forwarder pod already exists", "pod", foundPod.Name)
	return foundPod, nil
}


func (r *ServiceFunctionReconciler) createOrUpdateHostSFPod(ctx context.Context, instance *networkingv1alpha1.ServiceFunction) (*corev1.Pod, error) {
	log := logf.FromContext(ctx)

	podName := fmt.Sprintf("%s-hostsf", instance.Name)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: instance.Namespace,
			Labels: map[string]string{
				"app": "hostsf",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  instance.Spec.HostSF.Name,
					Image: instance.Spec.HostSF.Image,
					Ports: instance.Spec.HostSF.Ports,
				},
			},
			NodeSelector: instance.Spec.NodeSelector,
		},
	}

	if err := controllerutil.SetControllerReference(instance, pod, r.Scheme); err != nil {
		log.Error(err, "unable to set controller reference on hostsf pod")
		return nil, err
	}

	found := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating host service function pod", "pod", pod.Name)
		if err := r.Create(ctx, pod); err != nil {
			log.Error(err, "failed to create hostsf pod")
			return nil, err
		}
		return pod, nil
	} else if err != nil {
		log.Error(err, "failed to fetch hostsf pod")
		return nil, err
	}

	log.Info("Host service function pod already exists", "pod", found.Name)
	return found, nil
}

func isPodReady(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}


func (r *ServiceFunctionReconciler) getFunctionServiceInfo(ctx context.Context, namespace string, targetHost string) (string, int32, error) {
	var svcList corev1.ServiceList
	err := r.List(ctx, &svcList, &client.ListOptions{
		Namespace: namespace,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			"sfc.comnets.io/servicefunction": targetHost,
		}),
	})
	if err != nil {
		return "", 0, fmt.Errorf("failed to list services for function %s: %w", targetHost, err)
	}

	if len(svcList.Items) == 0 {
		return "", 0, fmt.Errorf("no Service found with label sfc.comnets.io/servicefunction=%s", targetHost)
	}
	if len(svcList.Items) > 1 {
		return "", 0, fmt.Errorf("multiple Services found with label sfc.comnets.io/servicefunction=%s", targetHost)
	}

	svc := svcList.Items[0]
	if len(svc.Spec.Ports) == 0 {
		return "", 0, fmt.Errorf("service %s has no ports defined", svc.Name)
	}

	return svc.Spec.ClusterIP, svc.Spec.Ports[0].Port, nil
}
