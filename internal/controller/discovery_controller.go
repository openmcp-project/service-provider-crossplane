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
	"encoding/json"
	"fmt"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/openmcp-project/controller-utils/pkg/clusters"

	v1alpha1 "github.com/openmcp-project/service-provider-crossplane/api/v1alpha1"
	"github.com/openmcp-project/service-provider-crossplane/internal/discovery"
)

// discoveryGVK is the GVK for the OCM Discovery resource.
var discoveryGVK = schema.GroupVersionKind{
	Group:   "delivery.ocm.software",
	Version: "v1alpha1",
	Kind:    "Discovery",
}

// DiscoveryReconciler reconciles the Discovery resources (delivery.ocm.software/v1alpha1) on the
// platform cluster that publish the available Crossplane and provider versions. It parses their
// status.discovery into the shared version Store and triggers reconciliation of all Crossplane
// instances when the available versions change.
type DiscoveryReconciler struct {
	PlatformCluster   *clusters.Cluster
	OnboardingCluster *clusters.Cluster
	VersionStore      *discovery.Store
	SendEventsChannel chan<- event.GenericEvent
	// ProviderName is the name of the ProviderConfig that holds the discovery references.
	ProviderName string
	// Namespace is the namespace on the platform cluster in which the Discovery resources live.
	Namespace string

	// ownedMu guards owned.
	ownedMu sync.Mutex
	// owned tracks, per Discovery resource, the set of short component names it last contributed to
	// the store.
	owned map[types.NamespacedName]map[string]struct{}
}

// +kubebuilder:rbac:groups=delivery.ocm.software,resources=discoveries,verbs=get;list;watch

// Reconcile parses a Discovery resource's status into the version Store and, when it is relevant
// to the ProviderConfig, triggers reconciliation of all Crossplane instances.
func (r *DiscoveryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	pc := &v1alpha1.ProviderConfig{}
	if err := r.PlatformCluster.Client().Get(ctx, client.ObjectKey{Name: r.ProviderName}, pc); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get ProviderConfig")
		return ctrl.Result{}, err
	}

	// Discovery sources only live in the service provider's own namespace.
	if r.Namespace != "" && req.Namespace != r.Namespace {
		return ctrl.Result{}, nil
	}

	changed, err := r.reconcileDiscovery(ctx, req.NamespacedName, pc)
	if err != nil {
		return ctrl.Result{}, err
	}
	if changed {
		r.triggerCrossplaneReconciles(ctx)
	}
	return ctrl.Result{}, nil
}

// reconcileDiscovery handles fetch, relevance check, and store update for a single Discovery.
// Returns true if the version store was modified.
func (r *DiscoveryReconciler) reconcileDiscovery(ctx context.Context, key types.NamespacedName, pc *v1alpha1.ProviderConfig) (bool, error) {
	disc, err := r.getDiscovery(ctx, key)
	if err != nil {
		return false, err
	}
	if disc == nil {
		return r.releaseOwned(key), nil
	}

	relevant, err := r.isRelevant(pc, disc.GetName(), disc.GetLabels())
	if err != nil {
		return false, err
	}
	if !relevant {
		return r.releaseOwned(key), nil
	}

	if err := r.applyDiscovery(ctx, disc); err != nil {
		return false, nil // parsing errors are not retryable
	}
	return true, nil
}

// getDiscovery fetches the Discovery resource, returning nil if not found.
func (r *DiscoveryReconciler) getDiscovery(ctx context.Context, key types.NamespacedName) (*unstructured.Unstructured, error) {
	disc := &unstructured.Unstructured{}
	disc.SetGroupVersionKind(discoveryGVK)
	if err := r.PlatformCluster.Client().Get(ctx, key, disc); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return disc, nil
}

// applyDiscovery parses a relevant Discovery resource and reconciles its contribution to the
// version Store.
func (r *DiscoveryReconciler) applyDiscovery(ctx context.Context, disc *unstructured.Unstructured) error {
	log := log.FromContext(ctx)

	components, err := parseDiscoveryStatus(disc)
	if err != nil {
		log.Error(err, "failed to parse Discovery resource", "discovery", disc.GetName())
		return err
	}

	byShortName := map[string][]discovery.Component{}
	for _, comp := range components {
		short := discovery.ShortName(comp.Name)
		byShortName[short] = append(byShortName[short], comp)
	}

	current := map[string]struct{}{}
	for short, comps := range byShortName {
		versions, pullSecrets := discovery.BuildComponentVersions(comps)
		r.VersionStore.Set(short, versions, pullSecrets)
		current[short] = struct{}{}
		log.Info("updated discovered versions", "component", short, "versionCount", len(versions))
	}

	// Delete short names this Discovery owned previously but no longer publishes.
	key := types.NamespacedName{Name: disc.GetName(), Namespace: disc.GetNamespace()}
	r.ownedMu.Lock()
	for short := range r.owned[key] {
		if _, ok := current[short]; !ok {
			r.VersionStore.Delete(short)
			log.Info("removed discovered versions", "component", short)
		}
	}
	if r.owned == nil {
		r.owned = map[types.NamespacedName]map[string]struct{}{}
	}
	r.owned[key] = current
	r.ownedMu.Unlock()
	return nil
}

// releaseOwned deletes from the store every short component name the given Discovery resource
// owned and forgets it. It reports whether anything was removed.
func (r *DiscoveryReconciler) releaseOwned(key types.NamespacedName) bool {
	r.ownedMu.Lock()
	owned := r.owned[key]
	delete(r.owned, key)
	r.ownedMu.Unlock()

	for short := range owned {
		r.VersionStore.Delete(short)
	}
	return len(owned) > 0
}

// isRelevant reports whether the Discovery with the given name and labels is a discovery source
// referenced by the ProviderConfig.
func (r *DiscoveryReconciler) isRelevant(pc *v1alpha1.ProviderConfig, name string, lbls map[string]string) (bool, error) {
	if name == pc.Spec.CrossplaneDiscoveryName {
		return true, nil
	}
	if lbls == nil {
		return false, nil
	}
	selector, err := metav1.LabelSelectorAsSelector(&pc.Spec.ProviderDiscoverySelector)
	if err != nil {
		return false, fmt.Errorf("invalid providerDiscoverySelector: %w", err)
	}
	// An empty selector matches everything; treat it as "no provider sources".
	if selector.Empty() {
		return false, nil
	}
	return selector.Matches(labels.Set(lbls)), nil
}

// triggerCrossplaneReconciles fans out a generic event for every Crossplane instance so they
// re-evaluate against the updated version Store.
func (r *DiscoveryReconciler) triggerCrossplaneReconciles(ctx context.Context) {
	log := log.FromContext(ctx)

	crossplanesList := &v1alpha1.CrossplaneList{}
	if err := r.OnboardingCluster.Client().List(ctx, crossplanesList); err != nil {
		log.Error(err, "failed to list Crossplane resources")
		return
	}

	for _, crossplane := range crossplanesList.Items {
		genericEvent := event.GenericEvent{Object: crossplane.DeepCopy()}
		select {
		case r.SendEventsChannel <- genericEvent:
		case <-time.After(1 * time.Second):
			log.Info("channel send timeout, dropping event", "name", crossplane.Name, "namespace", crossplane.Namespace)
		}
	}
}

// parseDiscoveryStatus extracts the component list from a Discovery resource's status.discovery.
// It handles the flat list format produced when discoveryFields are configured.
func parseDiscoveryStatus(disc *unstructured.Unstructured) ([]discovery.Component, error) {
	status, ok := disc.Object["status"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("discovery %q has no status", disc.GetName())
	}

	rawDiscovery, ok := status["discovery"]
	if !ok || rawDiscovery == nil {
		return nil, fmt.Errorf("discovery %q has no status.discovery", disc.GetName())
	}

	// Extract pull secret from effectiveOCMConfig (first Secret entry, if any).
	pullSecret := extractPullSecret(status)

	// The status.discovery is a flat list when discoveryFields are configured.
	discoveryList, ok := rawDiscovery.([]interface{})
	if !ok {
		return nil, fmt.Errorf("discovery %q status.discovery is not a list", disc.GetName())
	}

	// Marshal back to JSON and parse into typed entries.
	raw, err := json.Marshal(discoveryList)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal discovery entries: %w", err)
	}

	var entries []discovery.Entry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("failed to unmarshal discovery entries: %w", err)
	}

	if len(entries) == 0 {
		return nil, nil
	}

	return discovery.ToComponents(entries, pullSecret), nil
}

// extractPullSecret returns the name of the first Secret referenced in effectiveOCMConfig, if any.
func extractPullSecret(status map[string]interface{}) string {
	rawConfig, ok := status["effectiveOCMConfig"]
	if !ok {
		return ""
	}
	configs, ok := rawConfig.([]interface{})
	if !ok {
		return ""
	}
	for _, c := range configs {
		entry, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		kind, _ := entry["kind"].(string)
		name, _ := entry["name"].(string)
		if kind == "Secret" && name != "" {
			return name
		}
	}
	return ""
}

// enqueueNamespaceDiscoveries maps a ProviderConfig change to reconcile requests for every
// Discovery resource in the namespace, so a change to crossplaneDiscoveryName or
// providerDiscoverySelector re-evaluates which Discoveries are (no longer) relevant.
func (r *DiscoveryReconciler) enqueueNamespaceDiscoveries(ctx context.Context, _ *v1alpha1.ProviderConfig) []ctrl.Request {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   discoveryGVK.Group,
		Version: discoveryGVK.Version,
		Kind:    discoveryGVK.Kind + "List",
	})
	if err := r.PlatformCluster.Client().List(ctx, list, client.InNamespace(r.Namespace)); err != nil {
		log.FromContext(ctx).Error(err, "failed to list Discoveries for ProviderConfig change")
		return nil
	}
	reqs := make([]ctrl.Request, 0, len(list.Items))
	for i := range list.Items {
		reqs = append(reqs, ctrl.Request{NamespacedName: types.NamespacedName{
			Name:      list.Items[i].GetName(),
			Namespace: list.Items[i].GetNamespace(),
		}})
	}
	return reqs
}

// SetupWithManager sets up the controller with the Manager.
func (r *DiscoveryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	cache := r.PlatformCluster.Cluster().GetCache()

	discoveryObj := &unstructured.Unstructured{}
	discoveryObj.SetGroupVersionKind(discoveryGVK)

	return ctrl.NewControllerManagedBy(mgr).
		WatchesRawSource(source.Kind(cache, discoveryObj, &handler.TypedEnqueueRequestForObject[*unstructured.Unstructured]{})).
		WatchesRawSource(source.Kind(cache, &v1alpha1.ProviderConfig{}, handler.TypedEnqueueRequestsFromMapFunc(r.enqueueNamespaceDiscoveries))).
		Named("discovery").
		Complete(r)
}
