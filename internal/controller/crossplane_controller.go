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
	"errors"
	"time"

	"github.com/openmcp-project/controller-utils/pkg/clusters"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/openmcp-project/control-plane-operator/api/v1beta1"
	clustersv1alpha1 "github.com/openmcp-project/openmcp-operator/api/clusters/v1alpha1"
	"github.com/openmcp-project/openmcp-operator/lib/clusteraccess"
	"github.com/openmcp-project/openmcp-operator/lib/utils"

	v1alpha1 "github.com/openmcp-project/service-provider-crossplane/api/v1alpha1"
	"github.com/openmcp-project/service-provider-crossplane/internal/scheme"
	"github.com/openmcp-project/service-provider-crossplane/pkg/component"

	"github.com/openmcp-project/control-plane-operator/pkg/controlplane/targetrbac"
	"github.com/openmcp-project/control-plane-operator/pkg/utils/rcontext"
)

var (
	errComponentRemaining           = errors.New("at least one component is still installed")
	errFailedToCreateCPNamespace    = errors.New("failed to create namespace for ControlPlane")
	errFailedToBuildRESTConfig      = errors.New("failed to build REST config from ControlPlane target")
	errFailedToRemoteClient         = errors.New("failed to build client for ControlPlane target")
	errFailedToEnsureFluxKubeconfig = errors.New("failed to generate or save Flux kubeconfig")
	errFailedToApplyFluxRBAC        = errors.New("failed to apply Flux RBAC")
)

// CrossplaneReconciler reconciles a Crossplane object
type CrossplaneReconciler struct {
	PlatformCluster         *clusters.Cluster
	OnboardingCluster       *clusters.Cluster
	ClusterAccessReconciler clusteraccess.Reconciler
}

// +kubebuilder:rbac:groups=crossplane.services.openmcp.cloud,resources=crossplanes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=crossplane.services.openmcp.cloud,resources=crossplanes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=crossplane.services.openmcp.cloud,resources=crossplanes/finalizers,verbs=update
// Reconcile reconciles the Crossplane instance.
func (r *CrossplaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the Crossplane instance from the onboarding cluster
	crossplane := &v1alpha1.Crossplane{}
	if err := r.OnboardingCluster.Cluster().GetClient().Get(ctx, req.NamespacedName, crossplane); err != nil {
		log.Error(err, "unable to fetch Crossplane")
		return ctrl.Result{}, err
	}

	log.Info("Reconciling Crossplane", "name", &crossplane.Name, "namespace", &crossplane.Namespace)

	// Get ProviderConfig from Platform cluster
	providerConfig := &v1alpha1.ProviderConfig{}
	if err := r.PlatformCluster.Client().Get(ctx, types.NamespacedName{Name: "default"}, providerConfig); err != nil {
		log.Error(err, "unable to fetch ProviderConfig", "name", "default")
		// TODO: status update on Crossplane resource with "internal error occurred"?
		return ctrl.Result{}, err
	}

	// Handle ProviderConfig as ReleaseChannel
	resolverFn := r.GetResolverFunc(crossplane, providerConfig)
	ctx = rcontext.WithVersionResolver(ctx, resolverFn)

	// ensure namespace on platform cluster
	tenantNamespace := utils.StableRequestNamespace(req.Namespace)
	ctx = rcontext.WithTenantNamespace(ctx, tenantNamespace)

	// Create a new ClusterRequest/AccessRequest based on Crossplane instance
	mcpCluster, err := r.ClusterAccessReconciler.MCPCluster(ctx, req)
	if err != nil {
		log.Error(err, "failed to get MCP cluster for Crossplane instance")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Flux KubeConfig and RBAC
	if err := targetrbac.Apply(ctx, mcpCluster.Client(), v1beta1.ServiceAccountReference{
		Name:      "openmcp-flux-deployer",
		Namespace: "openmcp-system",
	}); err != nil {
		return ctrl.Result{}, errors.Join(errFailedToApplyFluxRBAC, err)
	}

	// Handle CreateOrUpdate
	//    1. Ensure finalizer is set
	//    2. New Juggler

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CrossplaneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.ClusterAccessReconciler = clusteraccess.NewClusterAccessReconciler(r.PlatformCluster.Client(), v1alpha1.GroupVersion.Group)
	r.ClusterAccessReconciler.
		WithMCPScheme(scheme.MCP).
		WithRetryInterval(10 * time.Second).
		WithMCPPermissions(getMCPPermissions())

	return ctrl.NewControllerManagedBy(mgr).
		WatchesRawSource(source.Kind(r.OnboardingCluster.Cluster().GetCache(), &v1alpha1.Crossplane{}, &handler.TypedEnqueueRequestForObject[*v1alpha1.Crossplane]{})).
		Named("crossplane").
		Complete(r)
}

func getMCPPermissions() []clustersv1alpha1.PermissionsRequest {
	defaultVerbs := []string{"get", "list", "watch", "create", "update", "patch", "delete"}

	return []clustersv1alpha1.PermissionsRequest{
		{
			Rules: []rbac.PolicyRule{
				{
					APIGroups: []string{"apiextensions.k8s.io"},
					Resources: []string{"customresourcedefinitions"},
					Verbs:     defaultVerbs,
				},
				{
					APIGroups: []string{""},
					Resources: []string{"secrets", "configmaps"},
					Verbs:     defaultVerbs,
				},
				{
					APIGroups: []string{""},
					Resources: []string{"serviceaccounts"},
					Verbs:     defaultVerbs,
				},
				{
					APIGroups: []string{""},
					Resources: []string{"serviceaccounts/token"},
					Verbs:     []string{"create"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"namespaces"},
					Verbs:     defaultVerbs,
				},
				{
					APIGroups: []string{"rbac.authorization.k8s.io"},
					Resources: []string{"clusterroles", "clusterrolebindings"},
					Verbs:     defaultVerbs,
				},
				{
					APIGroups: []string{""},
					Resources: []string{"events"},
					Verbs:     defaultVerbs,
				},
				{
					APIGroups: []string{"admissionregistration.k8s.io"},
					Resources: []string{"validatingwebhookconfigurations"},
					Verbs:     defaultVerbs,
				},
			},
		},
	}
}

// GetResolverFunc returns a VersionResolver.
// The VersionResolver is used to verify if the Crossplane instance configuration with its providers is valid.
// It checks the name and versions agains the configured v1alpha1.ProviderConfig on the Platform cluster.
// The function returns a v1beta1.VersionResolverFn that can be used to resolve the versions later in the reconcile loop.
func (r *CrossplaneReconciler) GetResolverFunc(config *v1alpha1.Crossplane, providerConfig *v1alpha1.ProviderConfig) v1beta1.VersionResolverFn {
	return func(componentName string, version string) (v1beta1.ComponentVersion, error) {
		// Check if Crossplane is installable
		if componentName == component.ComponentNameCrossplane {
			// Check if available version matches the requested version
			for _, availableVersion := range providerConfig.Spec.Chart.AvailableVersions {
				if availableVersion == version {
					return v1beta1.ComponentVersion{
						HelmRepo:  providerConfig.Spec.Chart.Repository,
						HelmChart: providerConfig.Spec.Chart.Name,
						Version:   version,
					}, nil
				}
			}
			return v1beta1.ComponentVersion{}, errors.New("requested version not available")
		}

		// Check if Provider is installable
		for _, provider := range providerConfig.Spec.AvailableProviders {
			if componentName == provider.Name {
				// Provider exists, now lets check if version is available
				for _, availableVersion := range provider.Versions {
					if availableVersion == version {
						return v1beta1.ComponentVersion{
							DockerRef: provider.Package,
							Version:   version,
						}, nil
					}
				}
			}
		}
		return v1beta1.ComponentVersion{}, errors.New("requested version not available")
	}
}
