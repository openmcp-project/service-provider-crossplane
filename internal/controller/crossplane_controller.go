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
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/openmcp-project/controller-utils/pkg/clusters"
	"github.com/openmcp-project/controller-utils/pkg/controller/smartrequeue"
	openmcpconsts "github.com/openmcp-project/openmcp-operator/api/constants"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	condApi "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/openmcp-project/control-plane-operator/api/v1beta1"
	clustersv1alpha1 "github.com/openmcp-project/openmcp-operator/api/clusters/v1alpha1"
	commonapi "github.com/openmcp-project/openmcp-operator/api/common"
	providersv1alpha1 "github.com/openmcp-project/openmcp-operator/api/provider/v1alpha1"
	"github.com/openmcp-project/openmcp-operator/lib/clusteraccess"
	libutils "github.com/openmcp-project/openmcp-operator/lib/utils"

	v1alpha1 "github.com/openmcp-project/service-provider-crossplane/api/v1alpha1"
	"github.com/openmcp-project/service-provider-crossplane/internal/scheme"
	"github.com/openmcp-project/service-provider-crossplane/pkg/component"
	sputils "github.com/openmcp-project/service-provider-crossplane/pkg/utils"

	"github.com/openmcp-project/control-plane-operator/pkg/juggler"
	"github.com/openmcp-project/control-plane-operator/pkg/juggler/fluxcd"
	"github.com/openmcp-project/control-plane-operator/pkg/juggler/object"
	cpoutils "github.com/openmcp-project/control-plane-operator/pkg/utils"
	"github.com/openmcp-project/control-plane-operator/pkg/utils/rcontext"
)

var (
	errComponentRemaining = errors.New("at least one component is still installed")

	// Finalizer for Crossplane instance resources
	Finalizer = providersv1alpha1.GroupVersion.Group + "/finalizers"

	controllerName = v1alpha1.GroupVersion.Group
)

const (
	requestSuffixMCP = "--mcp"
)

// CrossplaneReconciler reconciles a Crossplane object
type CrossplaneReconciler struct {
	PlatformCluster         *clusters.Cluster
	OnboardingCluster       *clusters.Cluster
	ClusterAccessReconciler clusteraccess.Reconciler
	Recorder                record.EventRecorder
	RequeueStore            *smartrequeue.Store
}

// Reconcile reconciles the Crossplane instance.
// +kubebuilder:rbac:groups=crossplane.services.openmcp.cloud,resources=crossplanes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=crossplane.services.openmcp.cloud,resources=crossplanes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=crossplane.services.openmcp.cloud,resources=crossplanes/finalizers,verbs=update
func (r *CrossplaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the Crossplane instance from the onboarding cluster
	crossplane := &v1alpha1.Crossplane{}
	if err := r.OnboardingCluster.Client().Get(ctx, req.NamespacedName, crossplane); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Get smart requeue entry for this object
	requeueEntry := r.RequeueStore.For(crossplane)
	ctx = smartrequeue.NewContext(ctx, requeueEntry)

	log.Info("Reconciling Crossplane", "name", crossplane.Name, "namespace", crossplane.Namespace)

	// Get ProviderConfig from Platform cluster
	pc := &v1alpha1.ProviderConfig{}
	if err := r.PlatformCluster.Client().Get(ctx, types.NamespacedName{Name: "default"}, pc); err != nil {
		return requeueEntry.Error(err)
	}

	// Setup reconciliation context
	ctx, err := r.setupReconciliationContext(ctx, req, pc)
	if err != nil {
		return requeueEntry.Error(err)
	}

	// Ensure finalizer is set
	if err := r.ensureFinalizer(ctx, crossplane); err != nil {
		return requeueEntry.Error(err)
	}

	// Setup cluster access
	mcpCluster, result, err := r.setupClusterAccess(ctx, req)
	if err != nil {
		return requeueEntry.Error(err)
	}
	if result != nil {
		return requeueEntry.Backoff()
	}

	// Setup Flux kubeconfig
	ctx, err = r.setupFluxKubeconfig(ctx, req)
	if err != nil {
		return requeueEntry.Error(err)
	}

	return r.reconcileCrossplaneInstance(ctx, mcpCluster.Client(), crossplane, pc)
}

func (r *CrossplaneReconciler) updateStatus(ctx context.Context, crossplane *v1alpha1.Crossplane, newConditions *[]metav1.Condition) {
	log := log.FromContext(ctx)
	changed := cpoutils.UpdateConditions(&crossplane.Status.Conditions, *newConditions)
	if changed {
		// TODO: do not try to update status when Crossplane CR is deleted
		if err := r.OnboardingCluster.Client().Status().Update(ctx, crossplane); err != nil {
			log.Error(err, "failed to update status from Crossplane CR at Onboarding cluster")
		}
	}
}

func (r *CrossplaneReconciler) setupReconciliationContext(ctx context.Context, req ctrl.Request, providerConfig *v1alpha1.ProviderConfig) (context.Context, error) {
	// Handle ProviderConfig as ReleaseChannel
	resolverFn := r.GetResolverFunc(providerConfig)
	ctx = rcontext.WithVersionResolver(ctx, resolverFn)

	// ensure namespace on platform cluster
	tenantNamespace, err := libutils.StableMCPNamespace(req.Name, req.Namespace)
	if err != nil {
		return ctx, fmt.Errorf("failed to determine stable namespace for Crossplane instance: %w", err)
	}
	ctx = rcontext.WithTenantNamespace(ctx, tenantNamespace)

	return ctx, nil
}

func (r *CrossplaneReconciler) setupClusterAccess(ctx context.Context, req ctrl.Request) (*clusters.Cluster, *ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Create ClusterRequest/AccessRequest
	res, err := r.ClusterAccessReconciler.Reconcile(ctx, req)
	if err != nil {
		log.Error(err, "failed to reconcile cluster access for crossplane instance")
		return nil, nil, err
	}

	// AccessRequest was created but not yet granted
	if res.RequeueAfter > 0 {
		result := ctrl.Result{RequeueAfter: res.RequeueAfter}
		return nil, &result, nil
	}

	// Get MCPCluster for Crossplane instance
	mcpCluster, err := r.ClusterAccessReconciler.MCPCluster(ctx, req)
	if err != nil {
		log.Error(err, "failed to get MCP cluster for Crossplane instance")
		result := ctrl.Result{RequeueAfter: 30 * time.Second}
		return nil, &result, nil
	}

	return mcpCluster, nil, nil
}

func (r *CrossplaneReconciler) setupFluxKubeconfig(ctx context.Context, req ctrl.Request) (context.Context, error) {
	tenantNamespace, err := libutils.StableMCPNamespace(req.Name, req.Namespace)
	if err != nil {
		return ctx, fmt.Errorf("failed to determine stable namespace for Crossplane instance: %w", err)
	}

	// Get MCP AccessRequest to use for Flux
	mcpAccessRequest := &clustersv1alpha1.AccessRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusteraccess.StableRequestName(controllerName, req) + requestSuffixMCP,
			Namespace: tenantNamespace,
		},
	}

	if err := r.PlatformCluster.Client().Get(ctx, client.ObjectKeyFromObject(mcpAccessRequest), mcpAccessRequest); err != nil {
		return ctx, fmt.Errorf("failed to get MCP AccessRequest: %w", err)
	}

	ctx = rcontext.WithFluxKubeconfigRef(ctx, (*corev1.SecretReference)(mcpAccessRequest.Status.SecretRef))
	return ctx, nil
}

func (r *CrossplaneReconciler) reconcileCrossplaneInstance(ctx context.Context, mcpClient client.Client, xp *v1alpha1.Crossplane, pc *v1alpha1.ProviderConfig) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	requeueEntry := smartrequeue.FromContext(ctx)

	// Handle deletion
	if !xp.DeletionTimestamp.IsZero() {
		return r.deleteCrossplaneInstance(ctx, mcpClient, xp, pc)
	}

	conditions, err := r.createOrUpdateCrossplaneInstance(ctx, mcpClient, xp, pc)
	if err != nil {
		log.Error(err, "failed to create or update Crossplane instance")
		return requeueEntry.Error(err)
	}

	// Update status with new conditions
	newConditions := []metav1.Condition{}
	for _, c := range conditions {
		condApi.SetStatusCondition(&newConditions, c)
	}

	r.updateStatus(ctx, xp, &newConditions)

	// Successfully reconciled - reset the requeue backoff
	return requeueEntry.Reset()
}

func (r *CrossplaneReconciler) deleteCrossplaneInstance(ctx context.Context, mcpClient client.Client, xp *v1alpha1.Crossplane, pc *v1alpha1.ProviderConfig) (ctrl.Result, error) {
	requeueEntry := smartrequeue.FromContext(ctx)

	if !r.hasFinalizer(xp) {
		// No finalizer, nothing to do - delete completed
		return requeueEntry.Never()
	}

	log := log.FromContext(ctx)

	conditions, err := r.deleteControlPlaneComponents(ctx, mcpClient, xp, pc)

	// Update status with current conditions
	newConditions := []metav1.Condition{}
	for _, c := range conditions {
		condApi.SetStatusCondition(&newConditions, c)
	}
	r.updateStatus(ctx, xp, &newConditions)

	if errors.Is(err, errComponentRemaining) {
		log.Info(err.Error())
		return requeueEntry.Backoff()
	}
	if err != nil {
		return requeueEntry.Error(err)
	}

	if err := r.removeFinalizer(ctx, xp); err != nil {
		return requeueEntry.Error(err)
	}

	// Deletion completed successfully
	return requeueEntry.Never()
}

func (r *CrossplaneReconciler) deleteControlPlaneComponents(ctx context.Context, mcpClient client.Client, xp *v1alpha1.Crossplane, pc *v1alpha1.ProviderConfig) ([]metav1.Condition, error) {
	// disable all components
	xpCopy := xp.DeepCopy()
	xpCopy.Spec = v1alpha1.CrossplaneSpec{}
	// disable imagePullSecrets from ProviderConfig
	pcCopy := pc.DeepCopy()
	pcCopy.Spec = v1alpha1.ProviderConfigSpec{}

	j, err := r.newJuggler(ctx, mcpClient, xpCopy, pcCopy)
	if err != nil {
		return nil, err
	}
	result := j.Reconcile(ctx)

	anyComponentRemaining := false
	for _, cr := range result {
		// do not count components that are marked as "keep on uninstall".
		if kou, ok := cr.Component.(juggler.KeepOnUninstall); ok && kou.KeepOnUninstall() {
			continue
		}
		// status must be "Disabled", otherwise the component is counted as "Remaining".
		if cr.Result != juggler.StatusDisabled {
			anyComponentRemaining = true
		}
	}

	conditions := []metav1.Condition{}
	for _, componentResult := range result {
		conditions = append(conditions, componentResult.ToCondition())
	}

	if anyComponentRemaining {
		return conditions, errComponentRemaining
	}

	return conditions, nil
}

func (r *CrossplaneReconciler) createOrUpdateCrossplaneInstance(ctx context.Context, mcpClient client.Client, xp *v1alpha1.Crossplane, pc *v1alpha1.ProviderConfig) ([]metav1.Condition, error) {
	j, err := r.newJuggler(ctx, mcpClient, xp, pc)
	if err != nil {
		return nil, err
	}
	result := j.Reconcile(ctx)

	conditions := []metav1.Condition{}
	for _, componentResult := range result {
		if !componentResult.Component.IsEnabled() && componentResult.Result == juggler.StatusDisabled {
			// Component is not enabled and has been successfully uninstalled (or has never been installed).
			// Don't output a condition in this case.
			continue
		}
		conditions = append(conditions, componentResult.ToCondition())
	}

	return conditions, nil
}

func (r *CrossplaneReconciler) newJuggler(ctx context.Context, mcpClient client.Client, xp *v1alpha1.Crossplane, pc *v1alpha1.ProviderConfig) (*juggler.Juggler, error) {
	logger := log.FromContext(ctx)
	var comps []juggler.Component
	jug := juggler.NewJuggler(logger, juggler.NewEventRecorder(r.Recorder, xp))

	// Validate that we have Crossplane versions configured
	if len(pc.Spec.CrossplaneVersions) == 0 {
		return nil, errors.New("no Crossplane versions configured in ProviderConfig")
	}

	xpContainerImagePullSecrets := discoverCrossplaneImagePullSecrets(pc.Spec.CrossplaneVersions)

	xpComp := &component.Crossplane{
		Config:               &xp.Spec,
		ImagePullSecretNames: extractSecretNames(xpContainerImagePullSecrets),
	}
	comps = append(comps, xpComp)

	// Add image pull secrets as components to be managed by the juggler.
	// Target secret namespace is crossplane-system
	podNs := os.Getenv(openmcpconsts.EnvVariablePodNamespace)
	if podNs == "" {
		return nil, fmt.Errorf("environment variable %s not set - cannot determine source namespace for secrets", openmcpconsts.EnvVariablePodNamespace)
	}
	for _, ps := range xpContainerImagePullSecrets {
		if ps.Name == "" {
			continue
		}
		sec := &component.Secret{
			SourceClient: r.PlatformCluster.Client(),
			Source: types.NamespacedName{
				Name:      ps.Name,
				Namespace: podNs,
			},
			Target: types.NamespacedName{
				Name:      ps.Name,
				Namespace: component.CrossplaneNamespace,
			},
			Enabled: xpComp.IsEnabled(),
		}
		comps = append(comps, sec)
	}

	// Add Helm chart pull secrets as components to be managed by the juggler
	// These are needed for pulling Crossplane Helm charts from private OCI registries
	xpHelmChartPullSecrets := discoverCrossplaneHelmChartPullSecrets(pc.Spec.CrossplaneVersions)
	for _, ps := range xpHelmChartPullSecrets {
		if ps.Name == "" {
			continue
		}
		sec := &component.PlatformSecret{
			SourceClient: r.PlatformCluster.Client(),
			Source: types.NamespacedName{
				Name:      ps.Name,
				Namespace: podNs,
			},
			Target: types.NamespacedName{
				Name:      ps.Name,
				Namespace: rcontext.TenantNamespace(ctx),
			},
			Enabled: xpComp.IsEnabled(),
		}
		comps = append(comps, sec)
	}

	if xp.Spec.Providers != nil {
		// Validate that we have provider configurations when providers are requested
		if len(pc.Spec.Providers.AvailableProviders) == 0 {
			return nil, errors.New("providers are specified in Crossplane instance but no available providers configured in ProviderConfig")
		}

		pullSecrets := convertImagePullSecrets(pc.Spec.Providers.ImagePullSecrets)
		for _, provider := range xp.Spec.Providers {
			xpp := &component.CrossplaneProvider{
				Config:      provider,
				Enabled:     xpComp.IsEnabled(),
				PullSecrets: pullSecrets,
			}
			comps = append(comps, xpp)
		}
	}

	jug.RegisterComponent(comps...)

	logger.V(1).Info("Registered components for Crossplane instance",
		"componentCount", len(comps),
		"crossplaneName", xp.Name,
		"crossplaneNamespace", xp.Namespace)

	r.registerReconcilers(jug, logger, mcpClient)

	if err := jug.RegisterOrphanedComponents(ctx); err != nil {
		return nil, err
	}

	return jug, nil
}

func convertImagePullSecrets(secrets []commonapi.LocalObjectReference) []corev1.LocalObjectReference {
	if secrets == nil {
		return nil
	}
	result := make([]corev1.LocalObjectReference, len(secrets))
	for i, secret := range secrets {
		result[i] = corev1.LocalObjectReference{
			Name: secret.Name,
		}
	}
	return result
}

// extractSecretNames extracts the names from a slice of LocalObjectReference
func extractSecretNames(secrets []commonapi.LocalObjectReference) []string {
	if len(secrets) == 0 {
		return nil
	}
	result := make([]string, 0, len(secrets))
	for _, secret := range secrets {
		if secret.Name != "" {
			result = append(result, secret.Name)
		}
	}
	return result
}

// deduplicateSecretRefs removes duplicate secret references based on name
func deduplicateSecretRefs(secrets []commonapi.LocalObjectReference) []commonapi.LocalObjectReference {
	if len(secrets) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	result := []commonapi.LocalObjectReference{}

	for _, secret := range secrets {
		if secret.Name != "" && !seen[secret.Name] {
			seen[secret.Name] = true
			result = append(result, secret)
		}
	}

	return result
}

func discoverCrossplaneHelmChartPullSecrets(xpversions []v1alpha1.CrossplaneVersion) []commonapi.LocalObjectReference {
	secrets := make([]commonapi.LocalObjectReference, 0, len(xpversions))
	for _, v := range xpversions {
		secrets = append(secrets, v.Chart.SecretRef)
	}
	return deduplicateSecretRefs(secrets)
}

func discoverCrossplaneImagePullSecrets(xpversions []v1alpha1.CrossplaneVersion) []commonapi.LocalObjectReference {
	secrets := make([]commonapi.LocalObjectReference, 0, len(xpversions))
	for _, v := range xpversions {
		secrets = append(secrets, v.Image.SecretRef)
	}
	return deduplicateSecretRefs(secrets)
}

func (r *CrossplaneReconciler) registerReconcilers(juggler *juggler.Juggler, logger logr.Logger, mcpClient client.Client) {
	fr := fluxcd.NewFluxReconciler(logger, r.PlatformCluster.Client(), mcpClient, sputils.LabelComponentName)
	fr.RegisterType(
		&component.Crossplane{},
	)
	juggler.RegisterReconciler(fr)

	or := object.NewReconciler(logger, mcpClient, sputils.LabelComponentName)
	or.RegisterType(
		&component.Secret{},
		&component.CrossplaneProvider{},
	)

	por := object.NewReconciler(logger, r.PlatformCluster.Client(), sputils.LabelComponentName)
	por.RegisterType(
		&component.PlatformSecret{},
	)
	juggler.RegisterReconciler(por)
}

// SetupWithManager sets up the controller with the Manager.
func (r *CrossplaneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.ClusterAccessReconciler = clusteraccess.NewClusterAccessReconciler(r.PlatformCluster.Client(), controllerName)
	r.ClusterAccessReconciler.
		WithMCPScheme(scheme.MCP).
		WithRetryInterval(10 * time.Second).
		WithMCPPermissions(getMCPPermissions()).
		WithMCPRoleRefs(getMCPRoleRefs()).
		SkipWorkloadCluster()

	// Initialize smart requeue store with sensible defaults:
	// - Min interval: 5 seconds (quick retry for transient issues)
	// - Max interval: 5 minutes (cap on maximum wait time)
	// - Multiplier: 2.0 (exponential backoff with doubling)
	r.RequeueStore = smartrequeue.NewStore(5*time.Second, 5*time.Minute, 2.0)

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Crossplane{}).
		Complete(r)
}

func getMCPPermissions() []clustersv1alpha1.PermissionsRequest {
	defaultVerbs := []string{"get", "list", "watch", "create", "update", "patch", "delete"}

	return []clustersv1alpha1.PermissionsRequest{
		{
			Rules: []rbac.PolicyRule{
				{
					APIGroups: []string{"*"},
					Resources: []string{"*"},
					Verbs:     defaultVerbs,
				},
			},
		},
	}
}

func getMCPRoleRefs() []commonapi.RoleRef {
	return []commonapi.RoleRef{
		{
			Kind: "ClusterRole",
			Name: "cluster-admin",
		},
	}
}

// GetResolverFunc is used to verify if the Crossplane instance configuration with its providers is valid.
// It checks the name and versions against the configured v1alpha1.ProviderConfig on the Platform cluster.
// The function returns a v1beta1.VersionResolverFn that can be used to resolve the versions later in the reconcile loop.
func (r *CrossplaneReconciler) GetResolverFunc(providerConfig *v1alpha1.ProviderConfig) v1beta1.VersionResolverFn {
	return func(componentName string, version string) (v1beta1.ComponentVersion, error) {
		// Check if Crossplane is installable
		if componentName == component.CrossplaneRelease {
			// Check if available version matches the requested version
			for _, availableVersion := range providerConfig.Spec.CrossplaneVersions {
				if availableVersion.Version == version {
					return v1beta1.ComponentVersion{
						OCIURL:    availableVersion.Chart.URL,
						DockerRef: availableVersion.Image.URL,
						Version:   version,
					}, nil
				}
			}
			return v1beta1.ComponentVersion{}, errors.New("requested version not available")
		}

		// Check if Provider is installable
		for _, provider := range providerConfig.Spec.Providers.AvailableProviders {
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

func (r *CrossplaneReconciler) ensureFinalizer(ctx context.Context, object client.Object) error {
	updated := controllerutil.AddFinalizer(object, Finalizer)
	if updated {
		return r.OnboardingCluster.Client().Update(ctx, object)
	}
	return nil
}

func (r *CrossplaneReconciler) removeFinalizer(ctx context.Context, object client.Object) error {
	updated := controllerutil.RemoveFinalizer(object, Finalizer)
	if updated {
		return r.OnboardingCluster.Client().Update(ctx, object)
	}
	return nil
}

func (r *CrossplaneReconciler) hasFinalizer(object client.Object) bool {
	return controllerutil.ContainsFinalizer(object, Finalizer)
}
