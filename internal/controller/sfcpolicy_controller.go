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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	networkingv1alpha1 "github.com/binhfdv/sfc-k8s-operator/api/v1alpha1"
)

// SFCPolicyReconciler reconciles a SFCPolicy object
type SFCPolicyReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=networking.sfc.comnets.com,resources=sfcpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.sfc.comnets.com,resources=sfcpolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.sfc.comnets.com,resources=sfcpolicies/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SFCPolicy object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.4/pkg/reconcile
func (r *SFCPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx)
	log := r.Log.WithValues("sfcpolicy", req.NamespacedName)

	// Fetch the SFCPolicy instance
	instance := &networkingv1alpha1.SFCPolicy{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		log.Error(err, "unable to fetch SFCPolicy")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// If the policy is already applied, we don't need to do anything further
	if instance.Status.Applied {
		log.Info("SFCPolicy already applied")
		return ctrl.Result{}, nil
	}

	// Fetch the related ServiceFunctionChain
	sfChain := &networkingv1alpha1.ServiceFunctionChain{}
	err = r.Get(ctx, types.NamespacedName{Name: instance.Spec.ApplyChain, Namespace: req.Namespace}, sfChain)
	if err != nil {
		log.Error(err, "unable to fetch ServiceFunctionChain", "chain", instance.Spec.ApplyChain)
		return ctrl.Result{}, err
	}

	// Log the ServiceFunctionChain status details for debugging purposes
	log.Info("ServiceFunctionChain status", "ready", sfChain.Status.Ready, "deployedPods", len(sfChain.Status.DeployedPods))

	// Check the match criteria (simple example)
	if sfChain.Status.Ready && len(sfChain.Status.DeployedPods) > 0 {
		// Policy is successfully applied
		instance.Status.Applied = true
		instance.Status.Reason = "Policy applied successfully"
		log.Info("Policy applied successfully", "chain", instance.Spec.ApplyChain)
	} else {
		// Policy application failed due to missing functions or chain not ready
		instance.Status.Applied = false
		instance.Status.Reason = "ServiceFunctionChain not ready"
		log.Info("ServiceFunctionChain not ready", "chain", instance.Spec.ApplyChain)
	}

	// Update the SFCPolicy status
	err = r.Status().Update(ctx, instance)
	if err != nil {
		log.Error(err, "unable to update SFCPolicy status")
		return ctrl.Result{}, err
	}

	log.Info("Reconciliation complete", "status", instance.Status)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SFCPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.SFCPolicy{}).
		Named("sfcpolicy").
		Complete(r)
}
