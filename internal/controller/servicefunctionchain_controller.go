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
	// logr "github.com/go-logr/logr"
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

// ServiceFunctionChainReconciler reconciles a ServiceFunctionChain object
type ServiceFunctionChainReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=networking.sfc.comnets.com,resources=servicefunctionchains,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.sfc.comnets.com,resources=servicefunctionchains/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.sfc.comnets.com,resources=servicefunctionchains/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ServiceFunctionChain object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.4/pkg/reconcile
func (r *ServiceFunctionChainReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// _ = logf.FromContext(ctx)
	// log := r.Log.WithValues("servicefunctionchain", req.NamespacedName)
	log := logf.FromContext(ctx).WithValues("servicefunctionchain", req.NamespacedName)
	log.Info("Reconcile called")

	// Fetch the ServiceFunctionChain instance
	instance := &networkingv1alpha1.ServiceFunctionChain{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		log.Error(err, "unable to fetch Service Function Chain")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log.Info("\n=========================================================\n" +
		"------------ Service Function Chain Controller ----------\n" +
		"=========================================================\n")

	deployedFunctions, err := r.checkServiceFunctionsReady(ctx, req.Namespace, instance.Spec.Functions)
	if err != nil {
		return ctrl.Result{}, err
	}

	foundPod, err := r.createOrUpdateINGRESSPod(ctx, instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Check Pod readiness before updating the Service Function Chain status
	isReady := false
	for _, cond := range foundPod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			isReady = true
			break
		}
	}

	if foundPod.Status.Phase == corev1.PodPending {
		log.Info("Pod is still pending. Waiting for containers to start...")
		return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
	}

	if foundPod.Status.Phase == corev1.PodFailed || foundPod.Status.Phase == corev1.PodUnknown {
		log.Error(nil, "INGRESS Pod failed or unknown, recreating", "status", foundPod.Status.Phase)

		err := r.Delete(ctx, foundPod)
		if err != nil {
			log.Error(err, "failed to delete unhealthy INGRESS pod")
			return ctrl.Result{}, err
		}

		return ctrl.Result{Requeue: true}, nil
	}

	updatedStatus := instance.Status.DeepCopy()
	updatedStatus.LastUpdated = metav1.Now()

	if isReady && foundPod.Status.Phase == corev1.PodRunning && len(deployedFunctions) == len(instance.Spec.Functions) {
		log.Info("All conditions met, marking Service Function Chain as Ready")
		updatedStatus.PodName = foundPod.Name
		updatedStatus.Ready = true
	} else {
		log.Info("Not all conditions met, marking Service Function Chain as Not Ready")
		updatedStatus.Ready = false
	}

	// Only update if something actually changed
	if instance.Status.Ready != updatedStatus.Ready ||
		instance.Status.PodName != updatedStatus.PodName ||
		!instance.Status.LastUpdated.Equal(&updatedStatus.LastUpdated) {

		instance.Status = *updatedStatus
		err = r.Status().Update(ctx, instance)
		if err != nil {
			log.Error(err, "unable to update Service Function Chain status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceFunctionChainReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.ServiceFunctionChain{}).
		Owns(&corev1.Pod{}).
		Named("servicefunctionchain").
		Complete(r)
}

func (r *ServiceFunctionChainReconciler) checkServiceFunctionsReady(ctx context.Context, namespace string, functionNames []string) ([]string, error) {
	log := logf.FromContext(ctx).WithValues("servicefunctionchain", namespace)

	var deployedFunctions []string

	for _, functionName := range functionNames {
		sf := &networkingv1alpha1.ServiceFunction{}
		err := r.Get(ctx, types.NamespacedName{Name: functionName, Namespace: namespace}, sf)
		if err != nil {
			if errors.IsNotFound(err) {
				log.Info("Service Function not found", "servicefunction", functionName)
			} else {
				log.Error(err, "unable to fetch Service Function", "servicefunction", functionName)
				return nil, err
			}
			continue
		}

		if sf.Status.Ready {
			deployedFunctions = append(deployedFunctions, functionName)
		} else {
			log.Info("Service Function not ready", "servicefunction", functionName, "status", sf.Status)
		}
	}

	return deployedFunctions, nil
}

func (r *ServiceFunctionChainReconciler) createOrUpdateINGRESSPod(ctx context.Context, instance *networkingv1alpha1.ServiceFunctionChain) (*corev1.Pod, error) {
	log := logf.FromContext(ctx).WithValues("servicefunctionchain", instance.Namespace)

	// Fetch the first service function's service info (IP and Port)
	clusterIP, port, err := r.getFirstFunctionServiceInfo(ctx, instance)
	if err != nil {
		log.Error(err, "failed to get first Service Function's service info")
		return nil, err
	}

	// Convert the port to string
	portStr := fmt.Sprintf("%d", port)

	println("========SFC CONTROLLER========")
	println("ingress name: ", instance.Name)
	println("SF - targetHost: ", clusterIP)
	println("SF - targetPort: ", portStr)

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
					Image: instance.Spec.Ingress.Image,
					Ports: instance.Spec.Ingress.Ports,
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
			NodeSelector: instance.Spec.Ingress.NodeSelector,
		},
	}

	// Set the owner reference so the controller is notified on pod changes (create, delete, update)
	if err := controllerutil.SetControllerReference(instance, pod, r.Scheme); err != nil {
		log.Error(err, "unable to set controller reference on pod")
		return nil, err
	}

	foundPod := &corev1.Pod{}
	err1 := r.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, foundPod)

	if err1 != nil && errors.IsNotFound(err1) {
		log.Info("Creating INGRESS pod", "pod", pod.Name)
		if err := r.Create(ctx, pod); err != nil {
			log.Error(err, "unable to create INGRESS pod", "pod", pod.Name)
			return nil, err
		}
		return pod, nil
	} else if err1 != nil {
		log.Error(err1, "unable to fetch INGRESS pod", "pod", pod.Name)
		return nil, err1
	}

	log.Info("INGRESS pod already exists", "pod", foundPod.Name)
	return foundPod, nil
}

func (r *ServiceFunctionChainReconciler) getFirstFunctionServiceInfo(ctx context.Context, instance *networkingv1alpha1.ServiceFunctionChain) (string, int32, error) {
	if len(instance.Spec.Functions) == 0 {
		return "", 0, fmt.Errorf("no service functions defined in the chain")
	}

	firstFunc := instance.Spec.Functions[0]

	var svcList corev1.ServiceList
	err := r.List(ctx, &svcList, &client.ListOptions{
		Namespace: instance.Namespace,
		LabelSelector: labels.SelectorFromSet(labels.Set{
			"sfc.comnets.io/servicefunction": firstFunc,
		}),
	})
	if err != nil {
		return "", 0, fmt.Errorf("failed to list services for function %s: %w", firstFunc, err)
	}

	if len(svcList.Items) == 0 {
		return "", 0, fmt.Errorf("no Service found with label sfc.comnets.io/servicefunction=%s", firstFunc)
	}

	if len(svcList.Items) > 1 {
		return "", 0, fmt.Errorf("multiple Services found with label sfc.comnets.io/servicefunction=%s", firstFunc)
	}

	svc := svcList.Items[0]
	if len(svc.Spec.Ports) == 0 {
		return "", 0, fmt.Errorf("service %s has no ports defined", svc.Name)
	}

	clusterIP := svc.Spec.ClusterIP
	port := svc.Spec.Ports[0].Port

	return clusterIP, port, nil
}
