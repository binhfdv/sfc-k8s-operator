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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

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

	foundPod, err := r.createOrUpdateForwarderPod(ctx, instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Check Pod readiness before updating the Service Function Chain status
	// You can check the pod's status or readiness using the `foundPod` object
	if foundPod.Status.Phase == corev1.PodRunning && len(deployedFunctions) == len(instance.Spec.Functions) {
		// Update the Service Function Chain status
		instance.Status.PodName = instance.Name // pod.name
		instance.Status.Ready = true            // You can add more checks here for pod readiness
	} else {
		instance.Status.Ready = false
	}

	// Update the status of the Service Function Chain
	err = r.Status().Update(ctx, instance)
	if err != nil {
		log.Error(err, "unable to update Service Function Chain status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceFunctionChainReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.ServiceFunctionChain{}).
		Watches(
			&networkingv1alpha1.ServiceFunction{},
			handler.EnqueueRequestsFromMapFunc(r.mapServiceFunctionToChains),
		).
		Watches(
			&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(r.mapPodToChains),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
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


func (r *ServiceFunctionChainReconciler) createOrUpdateForwarderPod(ctx context.Context, instance *networkingv1alpha1.ServiceFunctionChain,) (*corev1.Pod, error) {
	log := logf.FromContext(ctx).WithValues("servicefunctionchain", instance.Namespace)

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
					Image: instance.Spec.Forwarder.Image,
					Ports: instance.Spec.Forwarder.Ports,
				},
			},
			NodeSelector: instance.Spec.Forwarder.NodeSelector,
		},
	}

	foundPod := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, foundPod)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating FORWARDER pod", "pod", pod.Name)
		if err := r.Create(ctx, pod); err != nil {
			log.Error(err, "unable to create FORWARDER pod", "pod", pod.Name)
			return nil, err
		}
		return pod, nil
	} else if err != nil {
		log.Error(err, "unable to fetch FORWARDER pod", "pod", pod.Name)
		return nil, err
	}

	log.Info("FORWARDER pod already exists", "pod", foundPod.Name)
	return foundPod, nil
}


func (r *ServiceFunctionChainReconciler) mapServiceFunctionToChains(sf *networkingv1alpha1.ServiceFunction) ([]reconcile.Request, error) {
	ctx := context.Background()
	var result []reconcile.Request

	// List all ServiceFunctionChains in the same namespace
	var sfcList networkingv1alpha1.ServiceFunctionChainList
	if err := r.List(ctx, &sfcList, client.InNamespace(sf.Namespace)); err != nil {
		// log error but return empty so we don't panic in mapping
		return result
	}

	for _, sfc := range sfcList.Items {
		for _, fn := range sfc.Spec.Functions {
			if fn == sf.Name {
				result = append(result, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      sfc.Name,
						Namespace: sfc.Namespace,
					},
				})
				break // no need to check other functions
			}
		}
	}

	return result, nil
}

func (r *ServiceFunctionChainReconciler) mapPodToChains(obj client.Object) []ctrl.Request {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return nil
	}

	ctx := context.Background()
	var requests []ctrl.Request

	sfc := &networkingv1alpha1.ServiceFunctionChain{}
	err := r.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, sfc)
	if err == nil {
		requests = append(requests, ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      sfc.Name,
				Namespace: sfc.Namespace,
			},
		})
	}
	return requests
}
