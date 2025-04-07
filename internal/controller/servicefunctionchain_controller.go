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

	logr "github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
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
	Log    logr.Logger
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
	_ = logf.FromContext(ctx)
	log := r.Log.WithValues("servicefunctionchain", req.NamespacedName)

	// Fetch the ServiceFunctionChain instance
	instance := &networkingv1alpha1.ServiceFunctionChain{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		log.Error(err, "unable to fetch ServiceFunctionChain")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

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

	log.Info("Reconciliation complete", "deployedFunctions", deployedFunctions)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceFunctionChainReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.ServiceFunctionChain{}).
		Named("servicefunctionchain").
		Complete(r)
}
