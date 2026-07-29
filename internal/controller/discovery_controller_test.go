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
	"testing"

	"github.com/openmcp-project/controller-utils/pkg/clusters"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/openmcp-project/service-provider-crossplane/api/v1alpha1"
	"github.com/openmcp-project/service-provider-crossplane/internal/discovery"
	"github.com/openmcp-project/service-provider-crossplane/internal/scheme"
	"github.com/openmcp-project/service-provider-crossplane/pkg/component"
)

// newCrossplaneDiscovery builds an unstructured Discovery resource matching what
// the OCM controller publishes for Crossplane (flat list with discoveryFields).
func newCrossplaneDiscovery(ns string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "delivery.ocm.software/v1alpha1",
		"kind":       "Discovery",
		"metadata": map[string]interface{}{
			"name":      "crossplane-discovery",
			"namespace": ns,
		},
		"spec": map[string]interface{}{
			"discoveryFields": map[string]interface{}{
				"componentName":    "component.name",
				"componentVersion": "component.version",
				"imageRef":         "resource.access.imageReference",
				"name":             "resource.name",
			},
		},
		"status": map[string]interface{}{
			"discovery": []interface{}{
				map[string]interface{}{
					"componentName":    "github.com/openmcp-project/releasechannel/crossplane",
					"componentVersion": "v2.0.8",
					"imageRef":         "registry/charts/crossplane:2.0.8@sha256:aaa",
					"name":             "crossplane",
				},
				map[string]interface{}{
					"componentName":    "github.com/openmcp-project/releasechannel/crossplane",
					"componentVersion": "v2.0.8",
					"imageRef":         "registry/images/crossplane:v2.0.8@sha256:bbb",
					"name":             "image-crossplane",
				},
			},
			"effectiveOCMConfig": []interface{}{
				map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Secret",
					"name":       "xp-secret",
					"namespace":  "openmcp-system",
					"policy":     "Propagate",
				},
			},
		},
	}}
	return obj
}

// newProviderDiscovery builds an unstructured Discovery resource for a provider.
func newProviderDiscovery(ns string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "delivery.ocm.software/v1alpha1",
		"kind":       "Discovery",
		"metadata": map[string]interface{}{
			"name":      "provider-kubernetes-discovery",
			"namespace": ns,
			"labels": map[string]interface{}{
				"discovery": "provider",
			},
		},
		"spec": map[string]interface{}{
			"discoveryFields": map[string]interface{}{
				"componentName":    "component.name",
				"componentVersion": "component.version",
				"imageRef":         "resource.access.imageReference",
				"name":             "resource.name",
			},
		},
		"status": map[string]interface{}{
			"discovery": []interface{}{
				map[string]interface{}{
					"componentName":    "github.com/openmcp-project/releasechannel/provider-kubernetes",
					"componentVersion": "v1.2.1",
					"imageRef":         "registry/provider-kubernetes:v1.2.1@sha256:ccc",
					"name":             "provider-kubernetes",
				},
			},
		},
	}}
	return obj
}

func newDiscoveryReconciler(t *testing.T, store *discovery.Store, objs ...runtime.Object) *DiscoveryReconciler {
	t.Helper()

	pc := &v1alpha1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{Name: TestProviderName},
		Spec: v1alpha1.ProviderConfigSpec{
			CrossplaneDiscoveryName:   "crossplane-discovery",
			ProviderDiscoverySelector: metav1.LabelSelector{MatchLabels: map[string]string{"discovery": "provider"}},
		},
	}

	pb := fake.NewClientBuilder().WithScheme(scheme.Platform).WithObjects(pc)
	for _, obj := range objs {
		pb = pb.WithRuntimeObjects(obj)
	}
	platformClient := pb.Build()
	onboardingClient := fake.NewClientBuilder().WithScheme(scheme.Onboarding).Build()

	// Buffered so triggerCrossplaneReconciles never blocks (no Crossplane instances anyway).
	ch := make(chan event.GenericEvent, 16)

	return &DiscoveryReconciler{
		PlatformCluster:   clusters.NewTestClusterFromClient("platform", platformClient),
		OnboardingCluster: clusters.NewTestClusterFromClient("onboarding", onboardingClient),
		VersionStore:      store,
		SendEventsChannel: ch,
		ProviderName:      TestProviderName,
	}
}

func TestParseDiscoveryStatus(t *testing.T) {
	disc := newCrossplaneDiscovery("sp")
	comps, err := parseDiscoveryStatus(disc)
	require.NoError(t, err)
	require.Len(t, comps, 1)
	assert.Equal(t, "v2.0.8", comps[0].Version)
	assert.Equal(t, "xp-secret", comps[0].PullSecret)
	require.Len(t, comps[0].Resources, 2)

	// No status → error
	empty := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "delivery.ocm.software/v1alpha1",
		"kind":       "Discovery",
		"metadata":   map[string]interface{}{"name": "x", "namespace": "y"},
	}}
	_, err = parseDiscoveryStatus(empty)
	assert.Error(t, err)
}

func TestDiscoveryReconcile_Crossplane(t *testing.T) {
	store := discovery.NewStore()
	disc := newCrossplaneDiscovery("sp")
	r := newDiscoveryReconciler(t, store, disc)

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "crossplane-discovery", Namespace: "sp"}})
	require.NoError(t, err)

	cv, ok := store.Resolve(component.CrossplaneRelease, "v2.0.8")
	require.True(t, ok)
	assert.Equal(t, "registry/charts/crossplane:2.0.8@sha256:aaa", cv.OCIURL)
	assert.Equal(t, "registry/images/crossplane:v2.0.8@sha256:bbb", cv.DockerRef)
	assert.Equal(t, "xp-secret", store.PullSecret(component.CrossplaneRelease, "v2.0.8"))
}

func TestDiscoveryReconcile_ProviderBySelector(t *testing.T) {
	store := discovery.NewStore()
	disc := newProviderDiscovery("sp")
	r := newDiscoveryReconciler(t, store, disc)

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "provider-kubernetes-discovery", Namespace: "sp"}})
	require.NoError(t, err)

	cv, ok := store.Resolve("provider-kubernetes", "v1.2.1")
	require.True(t, ok)
	assert.Equal(t, "registry/provider-kubernetes:v1.2.1@sha256:ccc", cv.DockerRef)
}

func TestDiscoveryReconcile_IrrelevantIgnored(t *testing.T) {
	store := discovery.NewStore()
	disc := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "delivery.ocm.software/v1alpha1",
		"kind":       "Discovery",
		"metadata": map[string]interface{}{
			"name":      "unrelated",
			"namespace": "sp",
			"labels":    map[string]interface{}{"foo": "bar"},
		},
		"status": map[string]interface{}{
			"discovery": []interface{}{
				map[string]interface{}{
					"componentName":    "github.com/openmcp-project/releasechannel/provider-kubernetes",
					"componentVersion": "v1.2.1",
					"imageRef":         "registry/provider-kubernetes:v1.2.1@sha256:ccc",
					"name":             "provider-kubernetes",
				},
			},
		},
	}}
	r := newDiscoveryReconciler(t, store, disc)

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "unrelated", Namespace: "sp"}})
	require.NoError(t, err)

	_, ok := store.Resolve("provider-kubernetes", "v1.2.1")
	assert.False(t, ok)
}

func TestDiscoveryReconcile_Delete(t *testing.T) {
	store := discovery.NewStore()
	disc := newCrossplaneDiscovery("sp")
	r := newDiscoveryReconciler(t, store, disc)
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "crossplane-discovery", Namespace: "sp"}}

	// First reconcile populates the store and records ownership.
	_, err := r.Reconcile(context.Background(), req)
	require.NoError(t, err)
	_, ok := store.Resolve(component.CrossplaneRelease, "v2.0.8")
	require.True(t, ok)

	// Delete the Discovery from the client, then reconcile again: the entry it owned is dropped.
	require.NoError(t, r.PlatformCluster.Client().Delete(context.Background(), disc))
	_, err = r.Reconcile(context.Background(), req)
	require.NoError(t, err)

	_, ok = store.Resolve(component.CrossplaneRelease, "v2.0.8")
	assert.False(t, ok)
}

func TestDiscoveryReconcile_EmptyDiscoveryClearsStore(t *testing.T) {
	store := discovery.NewStore()
	disc := newCrossplaneDiscovery("sp")
	r := newDiscoveryReconciler(t, store, disc)
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "crossplane-discovery", Namespace: "sp"}}

	_, err := r.Reconcile(context.Background(), req)
	require.NoError(t, err)
	_, ok := store.Resolve(component.CrossplaneRelease, "v2.0.8")
	require.True(t, ok)

	// Update the Discovery to have an empty discovery list.
	current := &unstructured.Unstructured{}
	current.SetGroupVersionKind(discoveryGVK)
	require.NoError(t, r.PlatformCluster.Client().Get(context.Background(), req.NamespacedName, current))
	status := current.Object["status"].(map[string]interface{})
	status["discovery"] = []interface{}{}
	require.NoError(t, r.PlatformCluster.Client().Update(context.Background(), current))

	_, err = r.Reconcile(context.Background(), req)
	require.NoError(t, err)

	_, ok = store.Resolve(component.CrossplaneRelease, "v2.0.8")
	assert.False(t, ok)
	assert.Empty(t, store.AvailableVersions(component.CrossplaneRelease))
}

func TestDiscoveryReconcile_WrongNamespaceIgnored(t *testing.T) {
	store := discovery.NewStore()
	disc := newCrossplaneDiscovery("other")
	r := newDiscoveryReconciler(t, store, disc)
	r.Namespace = "sp" // only "sp" is the discovery namespace

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "crossplane-discovery", Namespace: "other"}})
	require.NoError(t, err)

	_, ok := store.Resolve(component.CrossplaneRelease, "v2.0.8")
	assert.False(t, ok)
}

func TestDiscoveryReconcile_EmptySelectorMatchesNothing(t *testing.T) {
	store := discovery.NewStore()
	disc := newProviderDiscovery("sp")
	r := newDiscoveryReconciler(t, store, disc)

	// Force an empty provider selector on the ProviderConfig.
	pc := &v1alpha1.ProviderConfig{}
	require.NoError(t, r.PlatformCluster.Client().Get(context.Background(), types.NamespacedName{Name: TestProviderName}, pc))
	pc.Spec.ProviderDiscoverySelector = metav1.LabelSelector{}
	require.NoError(t, r.PlatformCluster.Client().Update(context.Background(), pc))

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "provider-kubernetes-discovery", Namespace: "sp"}})
	require.NoError(t, err)

	_, ok := store.Resolve("provider-kubernetes", "v1.2.1")
	assert.False(t, ok)
}
