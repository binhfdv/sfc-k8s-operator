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

	// logr "github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

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

	// Fetch the ServiceFunctionChain instance
	instance := &networkingv1alpha1.ServiceFunctionChain{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		log.Error(err, "unable to fetch ServiceFunctionChain")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log.Info("\n=========================================================\n" +
		"------------ Service Function Chain Controller ----------\n" +
		"=========================================================\n")

	// Check the status of the functions in the chain
	var deployedFunctions []string
	for _, functionName := range instance.Spec.Functions {
		sf := &networkingv1alpha1.ServiceFunction{}
		err := r.Get(ctx, types.NamespacedName{Name: functionName, Namespace: req.Namespace}, sf)
		if err != nil {
			if errors.IsNotFound(err) {
				log.Info("ServiceFunction not found", "servicefunction", functionName)
			} else {
				log.Error(err, "unable to fetch ServiceFunction", "servicefunction", functionName)
			}
			continue // Proceed to the next function if this one fails to fetch
		}

		// Add the function name to deployed list if it is ready
		if sf.Status.Ready {
			deployedFunctions = append(deployedFunctions, functionName)
		} else {
			log.Info("ServiceFunction not ready", "servicefunction", functionName)
		}
	}

	// Update the ServiceFunctionChain status with the list of deployed service functions
	instance.Status.DeployedPods = deployedFunctions
	instance.Status.Ready = len(deployedFunctions) == len(instance.Spec.Functions) // All functions should be ready

	// Update the status of the ServiceFunctionChain resource
	err = r.Status().Update(ctx, instance)
	if err != nil {
		log.Error(err, "unable to update ServiceFunctionChain status")
		return ctrl.Result{}, err
	}

	log.Info("Reconciliation completed: Service functions are ready.", "deployedFunctions", deployedFunctions)

	/////////////////////////////////
	// deploy forwarder

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name, // Add a unique identifier
			Namespace: req.Namespace,
			Labels:    instance.Labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  instance.Name,
					Image: instance.Spec.Forwarder.Image,
					Ports: instance.Spec.Forwarder.Ports,
				},
			},
			NodeSelector: instance.Spec.Forwarder.NodeSelector,
		},
	}

	// Check if the pod already exists
	foundPod := &corev1.Pod{}
	err = r.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, foundPod)
	if err != nil && errors.IsNotFound(err) {
		// Pod doesn't exist, create it
		log.Info("Creating pod", "pod", pod.Name)
		err = r.Create(ctx, pod)
		if err != nil {
			log.Error(err, "unable to create pod", "pod", pod.Name)
			return ctrl.Result{}, err
		}
	} else if err != nil {
		// Error checking pod existence
		log.Error(err, "unable to fetch pod", "pod", pod.Name)
		return ctrl.Result{}, err
	} else {
		// Pod already exists, update status or skip creation
		log.Info("Pod already exists", "pod", pod.Name)
	}

	// Check Pod readiness before updating the ServiceFunction status
	// You can check the pod's status or readiness using the `foundPod` object
	if foundPod.Status.Phase == corev1.PodRunning {
		// Update the ServiceFunction status
		instance.Status.PodName = pod.Name
		instance.Status.Ready = true // You can add more checks here for pod readiness
	} else {
		instance.Status.Ready = false
	}

	// Update the status of the ServiceFunction
	err = r.Status().Update(ctx, instance)
	if err != nil {
		log.Error(err, "unable to update ServiceFunctionChain status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceFunctionChainReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.ServiceFunctionChain{}).
		Named("servicefunctionchain").
		Complete(r)
}
