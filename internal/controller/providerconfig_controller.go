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
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/openmcp-project/controller-utils/pkg/clusters"

	v1alpha1 "github.com/openmcp-project/service-provider-crossplane/api/v1alpha1"
)

// ProviderConfigReconciler reconciles a ProviderConfig object
type ProviderConfigReconciler struct {
	PlatformCluster   *clusters.Cluster
	OnboardingCluster *clusters.Cluster
	SendEventsChannel chan<- event.GenericEvent
}

// +kubebuilder:rbac:groups=crossplane.services.openmcp.cloud,resources=providerconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=crossplane.services.openmcp.cloud,resources=providerconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=crossplane.services.openmcp.cloud,resources=providerconfigs/finalizers,verbs=update

// Reconcile reconciles the ProviderConfig instance. It listens for changes to ProviderConfig resources and triggers reconciliation for all Crossplane resources when a change is detected.
func (r *ProviderConfigReconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	crossplanesList := &v1alpha1.CrossplaneList{}
	if err := r.OnboardingCluster.Client().List(ctx, crossplanesList); err != nil {
		log.Error(err, "failed to list Crossplane resources")
		return ctrl.Result{}, err
	}

	log.Info("ProviderConfig changed, triggering reconciliation for Crossplane resources")

	for _, crossplane := range crossplanesList.Items {
		genericEvent := event.GenericEvent{
			Object: crossplane.DeepCopy(),
		}

		select {
		case r.SendEventsChannel <- genericEvent:
			log.Info("Sent Crossplane event to channel", "name", crossplane.Name, "namespace", crossplane.Namespace)

		// we don't block when the channel is full.
		case <-time.After(1 * time.Second):
			log.Info("Channel send timeout, dropping event", "name", crossplane.Name, "namespace", crossplane.Namespace)
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProviderConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WatchesRawSource(source.Kind(r.PlatformCluster.Cluster().GetCache(), &v1alpha1.ProviderConfig{}, &handler.TypedEnqueueRequestForObject[*v1alpha1.ProviderConfig]{})).
		Named("providerconfig").
		Complete(r)
}
