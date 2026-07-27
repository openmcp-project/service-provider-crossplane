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
	"slices"
	"time"

	"github.com/go-logr/logr"
	"github.com/openmcp-project/controller-utils/pkg/clusters"
	"github.com/openmcp-project/controller-utils/pkg/conditions"
	ctrlutils "github.com/openmcp-project/controller-utils/pkg/controller"
	"github.com/openmcp-project/controller-utils/pkg/controller/smartrequeue"
	errutils "github.com/openmcp-project/controller-utils/pkg/errors"
	openmcpconsts "github.com/openmcp-project/openmcp-operator/api/constants"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/openmcp-project/control-plane-operator/api/v1beta1"
	clustersv1alpha1 "github.com/openmcp-project/openmcp-operator/api/clusters/v1alpha1"
	commonapi "github.com/openmcp-project/openmcp-operator/api/common"
	providersv1alpha1 "github.com/openmcp-project/openmcp-operator/api/provider/v1alpha1"
	"github.com/openmcp-project/openmcp-operator/lib/clusteraccess"
	libutils "github.com/openmcp-project/openmcp-operator/lib/utils"

	v1alpha1 "github.com/openmcp-project/service-provider-crossplane/api/v1alpha1"
	"github.com/openmcp-project/service-provider-crossplane/internal/scheme"
	"github.com/openmcp-project/service-provider-crossplane/pkg/component"
	"github.com/openmcp-project/service-provider-crossplane/pkg/crossplane"
	sputils "github.com/openmcp-project/service-provider-crossplane/pkg/utils"

	crossplanev1beta1 "github.com/crossplane/crossplane/apis/v2/pkg/v1beta1"
	"github.com/openmcp-project/control-plane-operator/pkg/controlplane/components"
	"github.com/openmcp-project/control-plane-operator/pkg/juggler"
	"github.com/openmcp-project/control-plane-operator/pkg/juggler/fluxcd"
	"github.com/openmcp-project/control-plane-operator/pkg/juggler/object"
	"github.com/openmcp-project/control-plane-operator/pkg/utils/rcontext"
)

var (
	errComponentRemaining = errors.New("at least one component is still installed")

	// FinalizerLegacy is the legacy finalizer for Crossplane instance resources.
	FinalizerLegacy = providersv1alpha1.GroupVersion.Group + "/finalizers"
	// FinalizerComponents is the finalizer for resources managed by the Juggler.
	FinalizerComponents = v1alpha1.GroupVersion.Group + "/components"
	// FinalizerAccess is the finalizer for access requests.
	FinalizerAccess = v1alpha1.GroupVersion.Group + "/mcp-access"

	// controllerName = v1alpha1.GroupVersion.Group
	// TODO: In order to avoid issues during the api group change with existing instances,
	// the name of the cluster access reconciler needs to stay the same.
	// This should be changed when there will be a solution for this.
	clusterAccessReconcilerName = "crossplane.services.openmcp.cloud"
)

const (
	requestSuffixMCP = "--mcp"
	secretNamePrefix = "sp-crossplane-"
)

// CrossplaneReconciler reconciles a Crossplane object
type CrossplaneReconciler struct {
	PlatformCluster         *clusters.Cluster
	OnboardingCluster       *clusters.Cluster
	ClusterAccessReconciler clusteraccess.Reconciler
	Recorder                events.EventRecorder
	RequeueStore            *smartrequeue.Store
	RecieveEventsChannel    <-chan event.GenericEvent
	SecretsNamespace        string
	ProviderName            string
}

// Reconcile reconciles the Crossplane instance.
// +kubebuilder:rbac:groups=crossplane.services.openmcp.cloud,resources=crossplanes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=crossplane.services.openmcp.cloud,resources=crossplanes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=crossplane.services.openmcp.cloud,resources=crossplanes/finalizers,verbs=update
func (r *CrossplaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	xp := &v1alpha1.Crossplane{}
	if err := r.OnboardingCluster.Client().Get(ctx, req.NamespacedName, xp); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	oldXP := xp.DeepCopy()

	log.Info("Reconciling Crossplane")

	rr := ctrlutils.ReconcileResult[*v1alpha1.Crossplane]{
		Object:    xp,
		OldObject: oldXP,
	}

	r.doReconcile(ctx, req, &rr)

	return ctrlutils.NewOpenMCPStatusUpdaterBuilder[*v1alpha1.Crossplane]().
		WithNestedStruct("Status").
		WithConditionUpdater(false).
		WithConditionEvents(r.Recorder, conditions.EventPerChange).
		WithPhaseUpdateFunc(computePhase).
		WithSmartRequeue(r.RequeueStore, smartRequeueConditional).
		Build().
		UpdateStatus(ctx, r.OnboardingCluster.Client(), rr)
}

func inDeletion(obj client.Object) bool {
	return !obj.GetDeletionTimestamp().IsZero()
}

// readyForCleanup checks if the Crossplane instance is in deletion and components have already been cleaned up successfully.
func (r *CrossplaneReconciler) readyForCleanup(crossplane *v1alpha1.Crossplane) bool {
	return inDeletion(crossplane) &&
		!r.hasFinalizer(crossplane, FinalizerComponents) &&
		!r.hasFinalizer(crossplane, FinalizerLegacy)
}

func (r *CrossplaneReconciler) doReconcile(ctx context.Context, req ctrl.Request, rr *ctrlutils.ReconcileResult[*v1alpha1.Crossplane]) {
	xp := rr.Object
	addCondition := ctrlutils.GenerateCreateConditionFunc(rr)

	// Get ProviderConfig from Platform cluster
	pc := &v1alpha1.ProviderConfig{}
	if err := r.PlatformCluster.Client().Get(ctx, types.NamespacedName{Name: r.ProviderName}, pc); err != nil {
		addCondition(ConditionTypeReconciled, metav1.ConditionFalse, ReasonProviderConfigNotFound, fmt.Sprintf("ProviderConfig '%s' not found: %v", r.ProviderName, err))
		rr.ReconcileError = errutils.WithReason(err, ReasonProviderConfigNotFound)
		r.Recorder.Eventf(xp, nil, corev1.EventTypeWarning, ReasonProviderConfigNotFound, "Reconcile", "ProviderConfig '%s' not found: %v", r.ProviderName, err)
		return
	}

	// Setup reconciliation context
	ctx, err := r.setupReconciliationContext(ctx, req, pc)
	if err != nil {
		addCondition(ConditionTypeReconciled, metav1.ConditionFalse, ReasonReconciliationContextFailed, err.Error())
		rr.ReconcileError = errutils.WithReason(err, ReasonReconciliationContextFailed)
		return
	}

	// Check if object is in deletion and everything except for the access request has been cleaned up already.
	if r.readyForCleanup(xp) {
		addCondition(ConditionTypeReconciled, metav1.ConditionFalse, ReasonDeletionInProgress, "Cleaning up cluster access")
		rr.SmartRequeue = ctrlutils.SR_RESET
		r.doCleanupClusterAccess(ctx, req, xp, rr)
		return
	}

	// Setup cluster access
	mcpCluster, result, err := r.setupClusterAccess(ctx, req, xp)
	if err != nil {
		addCondition(ConditionTypeReconciled, metav1.ConditionFalse, ReasonClusterAccessFailed, err.Error())
		rr.ReconcileError = errutils.WithReason(err, ReasonClusterAccessFailed)
		r.Recorder.Eventf(xp, nil, corev1.EventTypeWarning, ReasonClusterAccessFailed, "Reconcile", "Cluster access setup failed: %v", err)
		return
	}
	if result != nil {
		addCondition(ConditionTypeReconciled, metav1.ConditionFalse, ReasonClusterAccessPending, "Cluster access request pending")
		rr.SmartRequeue = ctrlutils.SR_BACKOFF
		rr.Result = *result
		return
	}

	// Setup Flux kubeconfig
	ctx, err = r.setupFluxKubeconfig(ctx, req)
	if err != nil {
		addCondition(ConditionTypeReconciled, metav1.ConditionFalse, ReasonFluxKubeconfigFailed, err.Error())
		rr.ReconcileError = errutils.WithReason(err, ReasonFluxKubeconfigFailed)
		return
	}

	// Pre-reconciliation steps succeeded
	addCondition(ConditionTypeReconciled, metav1.ConditionTrue, ReasonReconciled, "")

	// Reconcile components
	r.doReconcileComponents(ctx, mcpCluster.Client(), xp, pc, rr)
}

func (r *CrossplaneReconciler) doReconcileComponents(ctx context.Context, mcpClient client.Client, xp *v1alpha1.Crossplane, pc *v1alpha1.ProviderConfig, rr *ctrlutils.ReconcileResult[*v1alpha1.Crossplane]) {
	if inDeletion(xp) {
		r.doDeleteComponents(ctx, mcpClient, xp, pc, rr)
		return
	}

	// Add components finalizer
	if err := r.ensureFinalizer(ctx, xp, FinalizerComponents); err != nil {
		rr.ReconcileError = errutils.WithReason(fmt.Errorf("failed to add components finalizer: %w", err), ReasonFinalizerFailed)
		return
	}
	// Remove legacy finalizer
	if err := r.removeFinalizer(ctx, xp, FinalizerLegacy); err != nil {
		rr.ReconcileError = errutils.WithReason(fmt.Errorf("failed to remove legacy finalizer: %w", err), ReasonFinalizerFailed)
		return
	}

	componentConditions, err := r.createOrUpdateCrossplaneInstance(ctx, mcpClient, xp, pc)
	if err != nil {
		rr.ReconcileError = errutils.WithReason(errutils.IgnoreInvalidUserInput(err), ReasonComponentReconcileFailed)
		rr.Conditions = overrideCondition(rr.Conditions, ConditionTypeReconciled, metav1.ConditionFalse, ReasonComponentBuildFailed, err.Error())
		r.Recorder.Eventf(xp, nil, corev1.EventTypeWarning, ReasonComponentReconcileFailed, "Reconcile", "Failed to reconcile components: %v", err)
		return
	}

	rr.Conditions = append(rr.Conditions, componentConditions...)

	allReady := true
	for _, c := range componentConditions {
		if c.Status != metav1.ConditionTrue {
			allReady = false
			break
		}
	}
	if !allReady {
		rr.Conditions = overrideCondition(rr.Conditions, ConditionTypeReconciled, metav1.ConditionFalse, ReasonComponentReconcileFailed, "Not all components are ready")
	}

	// always do a backoff to not overload the reconciler
	// most of the times the components will become ready on the lower end of the back off
	// this should mostly handle crossplane instances that will never become ready and be stuck
	// the trade off is that the status update is not as instantaneous
	rr.SmartRequeue = ctrlutils.SR_BACKOFF
}

func (r *CrossplaneReconciler) doDeleteComponents(ctx context.Context, mcpClient client.Client, xp *v1alpha1.Crossplane, pc *v1alpha1.ProviderConfig, rr *ctrlutils.ReconcileResult[*v1alpha1.Crossplane]) {
	log := log.FromContext(ctx)

	if !r.hasFinalizer(xp, FinalizerComponents) {
		rr.SmartRequeue = ctrlutils.SR_RESET
		return
	}

	componentConditions, err := r.deleteControlPlaneComponents(ctx, mcpClient, xp, pc)
	rr.Conditions = append(rr.Conditions, componentConditions...)

	if errors.Is(err, errComponentRemaining) {
		log.Info(err.Error())
		rr.Conditions = overrideCondition(rr.Conditions, ConditionTypeReconciled, metav1.ConditionFalse, ReasonDeletionInProgress, "Deleting components")
		rr.SmartRequeue = ctrlutils.SR_BACKOFF
		return
	}
	if err != nil {
		rr.ReconcileError = errutils.WithReason(err, ReasonDeletionComponentCleanupError)
		rr.Conditions = overrideCondition(rr.Conditions, ConditionTypeReconciled, metav1.ConditionFalse, ReasonDeletionComponentCleanupError, err.Error())
		r.Recorder.Eventf(xp, nil, corev1.EventTypeWarning, ReasonDeletionComponentCleanupError, "Delete", "Failed to delete components: %v", err)
		return
	}

	// Remove components finalizer
	if err := r.removeFinalizer(ctx, xp, FinalizerComponents); err != nil {
		rr.ReconcileError = errutils.WithReason(fmt.Errorf("failed to remove components finalizer: %w", err), ReasonFinalizerFailed)
		return
	}

	rr.SmartRequeue = ctrlutils.SR_RESET
}

func (r *CrossplaneReconciler) doCleanupClusterAccess(ctx context.Context, req ctrl.Request, xp *v1alpha1.Crossplane, rr *ctrlutils.ReconcileResult[*v1alpha1.Crossplane]) {
	log := log.FromContext(ctx)

	// Delete AccessRequest
	res, err := r.ClusterAccessReconciler.ReconcileDelete(ctx, req)
	if err != nil {
		log.Error(err, "failed to delete cluster access for crossplane instance")
		rr.ReconcileError = errutils.WithReason(err, ReasonClusterAccessFailed)
		return
	}

	// AccessRequest was marked for deletion but not gone yet
	if res.RequeueAfter > 0 {
		rr.Result = res
		return
	}

	// Remove access finalizer
	if err := r.removeFinalizer(ctx, xp, FinalizerAccess); err != nil {
		log.Error(err, "failed to remove access finalizer")
		rr.ReconcileError = errutils.WithReason(err, ReasonFinalizerFailed)
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

func (r *CrossplaneReconciler) setupClusterAccess(ctx context.Context, req ctrl.Request, crossplane *v1alpha1.Crossplane) (*clusters.Cluster, *ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Add access finalizer
	if err := r.ensureFinalizer(ctx, crossplane, FinalizerAccess); err != nil {
		return nil, nil, err
	}
	// Remove legacy finalizer
	if err := r.removeFinalizer(ctx, crossplane, FinalizerLegacy); err != nil {
		return nil, nil, err
	}

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
			Name:      clusteraccess.StableRequestName(clusterAccessReconcilerName, req) + requestSuffixMCP,
			Namespace: tenantNamespace,
		},
	}

	if err := r.PlatformCluster.Client().Get(ctx, client.ObjectKeyFromObject(mcpAccessRequest), mcpAccessRequest); err != nil {
		return ctx, fmt.Errorf("failed to get MCP AccessRequest: %w", err)
	}

	secretRef := &corev1.SecretReference{
		Name:      mcpAccessRequest.Status.SecretRef.Name,
		Namespace: mcpAccessRequest.Namespace,
	}
	ctx = rcontext.WithFluxKubeconfigRef(ctx, secretRef)
	return ctx, nil
}

//nolint:gocyclo
func (r *CrossplaneReconciler) deleteControlPlaneComponents(ctx context.Context, mcpClient client.Client, xp *v1alpha1.Crossplane, pc *v1alpha1.ProviderConfig) ([]metav1.Condition, error) {
	// Use a tolerant version resolver during deletion: if a version is unknown,
	// return a placeholder so that components can still build their resource objects
	// (the Juggler identifies resources by name, not by URL content).
	ctx = rcontext.WithVersionResolver(ctx, tolerantResolverFunc(r.GetResolverFunc(pc)))

	// Phase 1: Delete providers and functions while Crossplane is still running.
	// Crossplane must be running to process Provider/Function finalizers; its PreUninstall
	// hook refuses to uninstall while Provider/ProviderRevision/Function/FunctionRevision resources exist.
	if len(xp.Spec.Providers) > 0 || len(xp.Spec.Functions) > 0 {
		phase1Comps := make([]juggler.Component, 0, len(xp.Spec.Providers)+len(xp.Spec.Functions))
		phase1Comps = append(phase1Comps, buildProviderComponents(xp, pc)...)
		phase1Comps = append(phase1Comps, buildFunctionComponents(xp, pc)...)
		conditions, remaining, err := r.deleteComponents(ctx, mcpClient, xp, phase1Comps, false)
		if err != nil {
			return nil, err
		}
		if remaining {
			return conditions, errComponentRemaining
		}
		// Providers/Functions are gone, but revisions may still be terminating.
		if hasOrphans, err := hasProviderRevisions(ctx, mcpClient); err != nil {
			return nil, err
		} else if hasOrphans {
			return conditions, errComponentRemaining
		}
		if hasOrphans, err := hasFunctionRevisions(ctx, mcpClient); err != nil {
			return nil, err
		} else if hasOrphans {
			return conditions, errComponentRemaining
		}
	}

	// Phase 2: All providers and functions are gone — delete everything else (Crossplane, secrets, DRC).
	comps, err := buildComponents(ctx, r.PlatformCluster.Client(), xp, pc, false)
	if err != nil {
		return nil, errors.Join(errors.New("failed to build components for Crossplane instance"), err)
	}
	conditions, remaining, err := r.deleteComponents(ctx, mcpClient, xp, comps, true)
	if err != nil {
		return nil, err
	}
	if remaining {
		return conditions, errComponentRemaining
	}
	return conditions, nil
}

func (r *CrossplaneReconciler) deleteComponents(ctx context.Context, mcpClient client.Client, xp *v1alpha1.Crossplane, comps []juggler.Component, ignoreKeepOnUninstall bool) ([]metav1.Condition, bool, error) {
	j, err := r.newJuggler(ctx, mcpClient, xp, comps)
	if err != nil {
		return nil, false, err
	}
	result := j.Reconcile(ctx)

	anyRemaining := false
	for _, cr := range result {
		if ignoreKeepOnUninstall {
			if kou, ok := cr.Component.(juggler.KeepOnUninstall); ok && kou.KeepOnUninstall() {
				continue
			}
		}
		if cr.Result != juggler.StatusDisabled {
			anyRemaining = true
		}
	}

	conditions := make([]metav1.Condition, 0, len(result))
	for _, cr := range result {
		conditions = append(conditions, cr.ToCondition())
	}
	return conditions, anyRemaining, nil
}

func hasProviderRevisions(ctx context.Context, mcpClient client.Client) (bool, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "pkg.crossplane.io",
		Version: "v1",
		Kind:    "ProviderRevision",
	})
	if err := mcpClient.List(ctx, list, client.Limit(1)); err != nil {
		if apimeta.IsNoMatchError(err) {
			return false, nil
		}
		return false, err
	}
	return len(list.Items) > 0, nil
}

func hasFunctionRevisions(ctx context.Context, mcpClient client.Client) (bool, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "pkg.crossplane.io",
		Version: "v1",
		Kind:    "FunctionRevision",
	})
	if err := mcpClient.List(ctx, list, client.Limit(1)); err != nil {
		if apimeta.IsNoMatchError(err) {
			return false, nil
		}
		return false, err
	}
	return len(list.Items) > 0, nil
}

func buildProviderComponents(xp *v1alpha1.Crossplane, pc *v1alpha1.ProviderConfig) []juggler.Component {
	comps := make([]juggler.Component, 0, len(xp.Spec.Providers))
	pullSecrets := convertImagePullSecrets(pc.Spec.Providers.ImagePullSecrets)
	for _, provider := range xp.Spec.Providers {
		comps = append(comps, &component.CrossplaneProvider{
			Config:      provider,
			Enabled:     false,
			PullSecrets: pullSecrets,
		})
	}
	return comps
}

func buildFunctionComponents(xp *v1alpha1.Crossplane, pc *v1alpha1.ProviderConfig) []juggler.Component {
	comps := make([]juggler.Component, 0, len(xp.Spec.Functions))
	pullSecrets := convertImagePullSecrets(functionImagePullSecrets(pc))
	for _, function := range xp.Spec.Functions {
		comps = append(comps, &component.CrossplaneFunction{
			Config:      function,
			Enabled:     false,
			PullSecrets: pullSecrets,
		})
	}
	return comps
}

// functionImagePullSecrets returns the image pull secrets for functions.
// If the functions section has its own secrets, those are used.
// Otherwise, the provider image pull secrets are used as a fallback.
func functionImagePullSecrets(pc *v1alpha1.ProviderConfig) []commonapi.LocalObjectReference {
	if len(pc.Spec.Functions.ImagePullSecrets) > 0 {
		return pc.Spec.Functions.ImagePullSecrets
	}
	return pc.Spec.Providers.ImagePullSecrets
}

func (r *CrossplaneReconciler) createOrUpdateCrossplaneInstance(ctx context.Context, mcpClient client.Client, xp *v1alpha1.Crossplane, pc *v1alpha1.ProviderConfig) ([]metav1.Condition, error) {
	comps, err := buildComponents(ctx, r.PlatformCluster.Client(), xp, pc, true)
	if err != nil {
		return nil, errors.Join(errors.New("failed to build components for Crossplane instance"), err)
	}
	j, err := r.newJuggler(ctx, mcpClient, xp, comps)
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

//nolint:gocyclo,prealloc
func (r *CrossplaneReconciler) newJuggler(ctx context.Context, mcpClient client.Client, xp *v1alpha1.Crossplane, components []juggler.Component) (*juggler.Juggler, error) {
	logger := log.FromContext(ctx)
	jug := juggler.NewJuggler(logger, juggler.NewEventRecorder(r.Recorder, xp))

	jug.RegisterComponent(components...)

	logger.V(1).Info("Registered components for Crossplane instance",
		"componentCount", len(components),
		"crossplaneName", xp.Name,
		"crossplaneNamespace", xp.Namespace)

	r.registerReconcilers(jug, logger, mcpClient)

	if err := jug.RegisterOrphanedComponents(ctx); err != nil {
		return nil, err
	}

	return jug, nil
}

// buildComponents builds the components for the Crossplane instance based on its spec and the ProviderConfig.
//
//nolint:gocyclo
func buildComponents(ctx context.Context, client client.Client, xp *v1alpha1.Crossplane, pc *v1alpha1.ProviderConfig, enabled bool) ([]juggler.Component, error) {
	comps := make([]juggler.Component, 0)

	podNs := os.Getenv(openmcpconsts.EnvVariablePodNamespace)
	if podNs == "" {
		return nil, fmt.Errorf("environment variable %s not set - cannot determine source namespace for secrets", openmcpconsts.EnvVariablePodNamespace)
	}

	xpHelmChartPullSecret, err := extractHelmChartPullSecretForVersion(xp.Spec.Version, pc.Spec.CrossplaneVersions)
	if err != nil && enabled {
		return nil, fmt.Errorf("failed to extract Crossplane Helm chart pull secret for version %s: %w", xp.Spec.Version, err)
	}
	xpContainerImagePullSecrets := discoverCrossplaneImagePullSecrets(xp.Spec, pc.Spec.CrossplaneVersions)

	prefixedChartPullSecret, err := prefixChartPullSecretName(xpHelmChartPullSecret)
	if err != nil {
		return nil, err
	}

	xpComp := &component.Crossplane{
		Enabled:              enabled,
		Config:               &xp.Spec,
		ChartPullSecretName:  prefixedChartPullSecret,
		ImagePullSecretNames: extractSecretNames(xpContainerImagePullSecrets),
		CABundleRef:          pc.Spec.CABundleRef,
	}
	comps = append(comps, xpComp)

	secretComps, err := buildAllSecretComponents(ctx, client, xpComp.IsEnabled(), xpHelmChartPullSecret, prefixedChartPullSecret, xpContainerImagePullSecrets, podNs, pc)
	if err != nil {
		return nil, err
	}
	comps = append(comps, secretComps...)

	if xp.Spec.Providers != nil {
		if len(pc.Spec.Providers.AvailableProviders) == 0 {
			return nil, errors.New("providers are specified in Crossplane instance but no available providers configured in ProviderConfig")
		}

		pullSecrets := convertImagePullSecrets(pc.Spec.Providers.ImagePullSecrets)
		for _, provider := range xp.Spec.Providers {
			comps = append(comps, &component.CrossplaneProvider{
				Config:      provider,
				Enabled:     xpComp.IsEnabled(),
				PullSecrets: pullSecrets,
			})
		}
	}

	if xp.Spec.Functions != nil {
		if len(pc.Spec.Functions.AvailableFunctions) == 0 {
			return nil, errors.New("functions are specified in Crossplane instance but no available functions configured in ProviderConfig")
		}

		pullSecrets := convertImagePullSecrets(functionImagePullSecrets(pc))
		for _, function := range xp.Spec.Functions {
			comps = append(comps, &component.CrossplaneFunction{
				Config:      function,
				Enabled:     xpComp.IsEnabled(),
				PullSecrets: pullSecrets,
			})
		}
	}

	// DeploymentRuntimeConfig "default" needs to exist even if config for custom CA is removed later.
	drc := &component.DeploymentRuntimeConfig{
		Enabled: xpComp.IsEnabled(),
		// TODO: will be fixed with https://github.com/openmcp-project/service-provider-crossplane/issues/176
		// nolint:goconst
		Name:   "default",
		Config: &crossplanev1beta1.DeploymentRuntimeConfigSpec{}, // empty by default,
	}
	comps = append(comps, drc)

	comps = append(comps, configureDRCForCustomCA(client, podNs, drc, pc, xpComp.IsEnabled())...)

	return comps, nil
}

func prefixChartPullSecretName(ref *commonapi.LocalObjectReference) (string, error) {
	if ref == nil || ref.Name == "" {
		return "", nil
	}
	name, err := prefixSecretName(ref.Name)
	if err != nil {
		return "", fmt.Errorf("error generating secret name: %w", err)
	}
	return name, nil
}

func buildAllSecretComponents(ctx context.Context, cl client.Client, enabled bool, chartPullSecret *commonapi.LocalObjectReference, prefixedChartPullSecret string, imagePullSecrets []commonapi.LocalObjectReference, podNs string, pc *v1alpha1.ProviderConfig) ([]juggler.Component, error) {
	comps := make([]juggler.Component, 0)
	comps = appendDistinct(comps, buildSecretsComponents(cl, imagePullSecrets, podNs, component.CrossplaneNamespace, enabled)...)
	comps = appendDistinct(comps, buildSecretsComponents(cl, pc.Spec.Providers.ImagePullSecrets, podNs, component.CrossplaneNamespace, enabled)...)
	comps = appendDistinct(comps, buildSecretsComponents(cl, functionImagePullSecrets(pc), podNs, component.CrossplaneNamespace, enabled)...)

	if chartPullSecret != nil && chartPullSecret.Name != "" {
		comps = append(comps, &component.PlatformSecret{
			SourceClient: cl,
			Source: types.NamespacedName{
				Name:      chartPullSecret.Name,
				Namespace: podNs,
			},
			Target: types.NamespacedName{
				Name:      prefixedChartPullSecret,
				Namespace: rcontext.TenantNamespace(ctx),
			},
			Enabled: enabled,
		})
	}
	return comps, nil
}

func configureDRCForCustomCA(client client.Client, podNs string, drc *component.DeploymentRuntimeConfig, pc *v1alpha1.ProviderConfig, enabled bool) []juggler.Component {
	comps := []juggler.Component{}

	if pc.Spec.CABundleRef != nil {
		cm := &component.ConfigMap{
			Enabled:      enabled,
			SourceClient: client,
			Source: types.NamespacedName{
				Name:      pc.Spec.CABundleRef.Name,
				Namespace: podNs,
			},
			Target: types.NamespacedName{
				Name:      crossplane.CABundleConfigMapName, // ConfigMap is always renamed to constant value
				Namespace: components.CrossplaneNamespace,
			},
		}
		comps = append(comps, cm)

		drc.Config.DeploymentTemplate = crossplane.GetDeploymentTemplateForCABundleRef(&corev1.ConfigMapKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: crossplane.CABundleConfigMapName, // ConfigMap is always renamed to constant value
			},
			Key: pc.Spec.CABundleRef.Key,
		})
	}

	return comps
}

func appendDistinct(slice []juggler.Component, elems ...juggler.Component) []juggler.Component {
	for _, elem := range elems {
		found := false
		for _, existing := range slice {
			// Check if components are the same based on their target name and namespace
			if getComponentKey(existing) == getComponentKey(elem) {
				found = true
				break
			}
		}
		if !found {
			slice = append(slice, elem)
		}
	}
	return slice
}

func getComponentKey(comp juggler.Component) string {
	// For secret components, create a unique key based on target name and namespace
	switch c := comp.(type) {
	case *component.Secret:
		return c.Target.Namespace + "/" + c.Target.Name
	case *component.PlatformSecret:
		return c.Target.Namespace + "/" + c.Target.Name
	default:
		// For other component types, use the component name
		return comp.GetName()
	}
}

func buildSecretsComponents(c client.Client, secretRefs []commonapi.LocalObjectReference, sourceNamespace string, targetNamespace string, enabled bool) []juggler.Component {
	secrets := make([]juggler.Component, 0, len(secretRefs))
	for _, ps := range secretRefs {
		if ps.Name == "" {
			continue
		}
		secret := &component.Secret{
			SourceClient: c,
			Source: types.NamespacedName{
				Name:      ps.Name,
				Namespace: sourceNamespace,
			},
			Target: types.NamespacedName{
				Name:      ps.Name,
				Namespace: targetNamespace,
			},
			Enabled: enabled,
		}
		secrets = append(secrets, secret)
	}
	return secrets
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

func extractHelmChartPullSecretForVersion(targetVersion string, xpversions []v1alpha1.CrossplaneVersion) (*commonapi.LocalObjectReference, error) {
	for _, v := range xpversions {
		if targetVersion == v.Version {
			return &v.Chart.SecretRef, nil
		}
	}
	available := make([]string, 0, len(xpversions))
	for _, v := range xpversions {
		available = append(available, v.Version)
	}
	return nil, fmt.Errorf("%w: requested version %q is not available, available versions: %v", errutils.ErrInvalidUserInput, targetVersion, available)
}

func discoverCrossplaneImagePullSecrets(spec v1alpha1.CrossplaneSpec, xpversions []v1alpha1.CrossplaneVersion) []commonapi.LocalObjectReference {
	secrets := make([]commonapi.LocalObjectReference, 0, len(xpversions))
	for _, v := range xpversions {
		if spec.Version == v.Version && v.Image != nil {
			secrets = append(secrets, v.Image.SecretRef)
		}
	}
	return deduplicateSecretRefs(secrets)
}

func (r *CrossplaneReconciler) registerReconcilers(juggler *juggler.Juggler, logger logr.Logger, mcpClient client.Client) {
	fr := fluxcd.NewFluxReconciler(logger, r.PlatformCluster.Client(), mcpClient, sputils.LabelComponentName).
		WithLabelFunc(sputils.LabelFunc(sputils.LabelManagedByValue))
	fr.RegisterType(
		&component.Crossplane{},
	)
	juggler.RegisterReconciler(fr)

	or := object.NewReconciler(logger, mcpClient, sputils.LabelComponentName).
		WithLabelFunc(sputils.LabelFunc(sputils.LabelManagedByValue))
	or.RegisterType(
		&component.Secret{},
		&component.ConfigMap{},
		&component.CrossplaneProvider{},
		&component.CrossplaneFunction{},
		&component.DeploymentRuntimeConfig{},
	)
	juggler.RegisterReconciler(or)

	por := object.NewReconciler(logger, r.PlatformCluster.Client(), sputils.LabelComponentName).
		WithLabelFunc(sputils.LabelFunc(sputils.LabelManagedByValue))
	por.RegisterType(
		&component.PlatformSecret{},
	)
	juggler.RegisterReconciler(por)
}

// SetupWithManager sets up the controller with the Manager.
func (r *CrossplaneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.ClusterAccessReconciler = clusteraccess.NewClusterAccessReconciler(r.PlatformCluster.Client(), clusterAccessReconcilerName)
	r.ClusterAccessReconciler.
		WithMCPScheme(scheme.MCP).
		WithRetryInterval(10 * time.Second).
		WithMCPPermissions(getMCPPermissions()).
		WithMCPRoleRefs(getMCPRoleRefs()).
		SkipWorkloadCluster()

	// Initialize smart requeue store with sensible defaults:
	// - Min interval: 1 seconds (quick retry for transient issues)
	// - Max interval: 5 minutes (cap on maximum wait time)
	// - Multiplier: 1.3 (exponential backoff)
	r.RequeueStore = smartrequeue.NewStore(1*time.Second, 5*time.Minute, 1.3)

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Crossplane{}).
		WatchesRawSource(source.Channel(r.RecieveEventsChannel, &handler.EnqueueRequestForObject{})).
		WatchesRawSource(source.Kind(
			r.PlatformCluster.Cluster().GetCache(),
			&corev1.Secret{},
			handler.TypedEnqueueRequestsFromMapFunc(r.mapSecretToRequests),
			ctrlutils.ToTypedPredicate[*corev1.Secret](
				predicate.NewPredicateFuncs(func(obj client.Object) bool {
					return obj.GetNamespace() == r.SecretsNamespace
				}),
			),
		)).
		Complete(r)
}

func (r *CrossplaneReconciler) mapSecretToRequests(ctx context.Context, secret *corev1.Secret) []ctrl.Request {
	log := log.FromContext(ctx)

	providerConfig := &v1alpha1.ProviderConfig{}
	if err := r.PlatformCluster.Client().Get(ctx, types.NamespacedName{Name: r.ProviderName}, providerConfig); err != nil {
		log.Error(err, "failed to get ProviderConfig while mapping secret")
		return nil
	}

	if !isSecretReferencedInProviderConfig(providerConfig, secret.Name) {
		return nil
	}

	crossplanesList := &v1alpha1.CrossplaneList{}
	if err := r.OnboardingCluster.Client().List(ctx, crossplanesList); err != nil {
		log.Error(err, "failed to list Crossplane resources")
		return nil
	}

	log.Info("Source secret changed, triggering reconciliation", "secret", secret.Name)

	requests := make([]ctrl.Request, 0, len(crossplanesList.Items))
	for _, crossplaneInstance := range crossplanesList.Items {
		requests = append(requests, ctrl.Request{
			NamespacedName: types.NamespacedName{Name: crossplaneInstance.Name, Namespace: crossplaneInstance.Namespace},
		})
	}
	return requests
}

func isSecretReferencedInProviderConfig(pc *v1alpha1.ProviderConfig, secretName string) bool {
	for _, versions := range pc.Spec.CrossplaneVersions {
		if versions.Chart.SecretRef.Name == secretName {
			return true
		}
		if versions.Image != nil && versions.Image.SecretRef.Name == secretName {
			return true
		}
	}
	for _, pullSecret := range pc.Spec.Providers.ImagePullSecrets {
		if pullSecret.Name == secretName {
			return true
		}
	}
	for _, pullSecret := range functionImagePullSecrets(pc) {
		if pullSecret.Name == secretName {
			return true
		}
	}
	return false
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

// GetResolverFunc is used to verify if the Crossplane instance configuration with its providers and functions is valid.
// It checks the name and versions against the configured v1alpha1.ProviderConfig on the Platform cluster.
// The function returns a v1beta1.VersionResolverFn that can be used to resolve the versions later in the reconcile loop.
//
//nolint:gocyclo
func (r *CrossplaneReconciler) GetResolverFunc(providerConfig *v1alpha1.ProviderConfig) v1beta1.VersionResolverFn {
	return func(componentName string, version string) (v1beta1.ComponentVersion, error) {
		// Check if Crossplane is installable
		if componentName == component.CrossplaneRelease {
			for _, availableVersion := range providerConfig.Spec.CrossplaneVersions {
				if availableVersion.Version == version {
					dockerRef := ""
					if availableVersion.Image != nil {
						dockerRef = availableVersion.Image.URL
					}
					return v1beta1.ComponentVersion{
						OCIURL:    availableVersion.Chart.URL, // format: <image-location>:<version>
						DockerRef: dockerRef,                  // format: <image-location>:<version>, empty if not specified
						Version:   version,
					}, nil
				}
			}
			available := make([]string, 0, len(providerConfig.Spec.CrossplaneVersions))
			for _, v := range providerConfig.Spec.CrossplaneVersions {
				available = append(available, v.Version)
			}
			return v1beta1.ComponentVersion{}, fmt.Errorf("%w: requested version %q is not available, available versions: %v", errutils.ErrInvalidUserInput, version, available)
		}

		// Check if Provider is installable
		var availableVersions []string
		for _, provider := range providerConfig.Spec.Providers.AvailableProviders {
			if componentName == provider.Name {
				if slices.Contains(provider.Versions, version) {
					return v1beta1.ComponentVersion{
						DockerRef: provider.Package + ":" + version, // format: <image-location>:<version>
						Version:   version,
					}, nil
				}
				availableVersions = append(availableVersions, provider.Versions...)
			}
		}
		if len(availableVersions) > 0 {
			return v1beta1.ComponentVersion{}, fmt.Errorf("%w: requested version %q for provider %q is not available, available versions: %v", errutils.ErrInvalidUserInput, version, componentName, availableVersions)
		}

		// Check if Function is installable
		var availableFunctionVersions []string
		for _, function := range providerConfig.Spec.Functions.AvailableFunctions {
			if componentName == function.Name {
				if slices.Contains(function.Versions, version) {
					return v1beta1.ComponentVersion{
						DockerRef: function.Package + ":" + version, // format: <image-location>:<version>
						Version:   version,
					}, nil
				}
				availableFunctionVersions = append(availableFunctionVersions, function.Versions...)
			}
		}
		if len(availableFunctionVersions) > 0 {
			return v1beta1.ComponentVersion{}, fmt.Errorf("%w: requested version %q for function %q is not available, available versions: %v", errutils.ErrInvalidUserInput, version, componentName, availableFunctionVersions)
		}
		return v1beta1.ComponentVersion{}, fmt.Errorf("%w: component %q is not available in the ProviderConfig", errutils.ErrInvalidUserInput, componentName)
	}
}

const placeholderOCIURL = "placeholder:deletion"

func tolerantResolverFunc(fn v1beta1.VersionResolverFn) v1beta1.VersionResolverFn {
	return func(componentName string, version string) (v1beta1.ComponentVersion, error) {
		comp, err := fn(componentName, version)
		if err != nil {
			return v1beta1.ComponentVersion{
				OCIURL:    placeholderOCIURL,
				DockerRef: placeholderOCIURL,
				Version:   version,
			}, nil
		}
		return comp, nil
	}
}

func (r *CrossplaneReconciler) ensureFinalizer(ctx context.Context, object client.Object, finalizer string) error {
	updated := controllerutil.AddFinalizer(object, finalizer)
	if updated {
		return r.OnboardingCluster.Client().Update(ctx, object)
	}
	return nil
}

func (r *CrossplaneReconciler) removeFinalizer(ctx context.Context, object client.Object, finalizer string) error {
	updated := controllerutil.RemoveFinalizer(object, finalizer)
	if updated {
		return r.OnboardingCluster.Client().Update(ctx, object)
	}
	return nil
}

func (r *CrossplaneReconciler) hasFinalizer(object client.Object, finalizer string) bool {
	return controllerutil.ContainsFinalizer(object, finalizer)
}

// prefixSecretName adds the "sp-crossplane-" prefix to the given secret name
// to prevent name collisions in namespaces where multiple service providers operate.
// If the resulting name exceeds 63 characters (K8s limit), it will be truncated
// and a hash suffix appended for uniqueness via ShortenToXCharacters.
func prefixSecretName(secretName string) (string, error) {
	return ctrlutils.ShortenToXCharacters(fmt.Sprintf("%s%s", secretNamePrefix, secretName), ctrlutils.K8sMaxNameLength)
}
