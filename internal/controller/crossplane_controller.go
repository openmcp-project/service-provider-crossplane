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

	"github.com/go-logr/logr"
	"github.com/openmcp-project/controller-utils/pkg/clusters"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	condApi "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/openmcp-project/control-plane-operator/api/v1beta1"
	clustersv1alpha1 "github.com/openmcp-project/openmcp-operator/api/clusters/v1alpha1"
	"github.com/openmcp-project/openmcp-operator/lib/clusteraccess"
	"github.com/openmcp-project/openmcp-operator/lib/utils"

	v1alpha1 "github.com/openmcp-project/service-provider-crossplane/api/v1alpha1"
	"github.com/openmcp-project/service-provider-crossplane/internal/scheme"
	"github.com/openmcp-project/service-provider-crossplane/pkg/component"

	"github.com/openmcp-project/control-plane-operator/pkg/controlplane/kubeconfiggen"
	"github.com/openmcp-project/control-plane-operator/pkg/controlplane/targetrbac"
	"github.com/openmcp-project/control-plane-operator/pkg/juggler"
	"github.com/openmcp-project/control-plane-operator/pkg/juggler/fluxcd"
	"github.com/openmcp-project/control-plane-operator/pkg/utils/rcontext"
)

var (
	errFailedToEnsureFluxKubeconfig = errors.New("failed to generate or save Flux kubeconfig")
	errFailedToApplyFluxRBAC        = errors.New("failed to apply Flux RBAC")
	errInvalidExpirationOrBuffer    = errors.New("desired expiration and buffer are incompatible. make sure that desired expiration is greater than the buffer")
)

const (
	keyKubeconfig = "kubeconfig"
	keyExpiration = "expiresAt"

	kubeconfigExpiration = 10 * time.Minute
	kubeconfigBuffer     = 3 * time.Minute
)

// CrossplaneReconciler reconciles a Crossplane object
type CrossplaneReconciler struct {
	PlatformCluster         *clusters.Cluster
	OnboardingCluster       *clusters.Cluster
	ClusterAccessReconciler clusteraccess.Reconciler
	Kubeconfiggen           kubeconfiggen.Generator
	Recorder                record.EventRecorder
}

// Reconcile reconciles the Crossplane instance.
// +kubebuilder:rbac:groups=crossplane.services.openmcp.cloud,resources=crossplanes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=crossplane.services.openmcp.cloud,resources=crossplanes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=crossplane.services.openmcp.cloud,resources=crossplanes/finalizers,verbs=update
func (r *CrossplaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	newConditions := []metav1.Condition{}

	// Fetch the Crossplane instance from the onboarding cluster
	crossplane := &v1alpha1.Crossplane{}
	if err := r.OnboardingCluster.Client().Get(ctx, req.NamespacedName, crossplane); err != nil {
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
	resolverFn := r.GetResolverFunc(providerConfig)
	ctx = rcontext.WithVersionResolver(ctx, resolverFn)

	// ensure namespace on platform cluster
	tenantNamespace := utils.StableRequestNamespace(req.Namespace)
	ctx = rcontext.WithTenantNamespace(ctx, tenantNamespace)

	// Create ClusterRequest/AccessRequest
	res, err := r.ClusterAccessReconciler.Reconcile(ctx, req)
	if err != nil {
		log.Error(err, "failed to reconcile cluster access for crossplane instance")
		return ctrl.Result{}, err
	}

	// AccessRequest was created but not yet granted
	if res.RequeueAfter > 0 {
		return ctrl.Result{RequeueAfter: res.RequeueAfter}, nil
	}

	// Get MCPCluster for Crossplane instance
	mcpCluster, err := r.ClusterAccessReconciler.MCPCluster(ctx, req)
	if err != nil {
		log.Error(err, "failed to get MCP cluster for Crossplane instance")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Flux KubeConfig and RBAC TODO: Is this really needed?
	if err := targetrbac.Apply(ctx, mcpCluster.Client(), v1beta1.ServiceAccountReference{
		Name:      "openmcp-flux-deployer",
		Namespace: "openmcp-system",
	}); err != nil {
		return ctrl.Result{}, errors.Join(errFailedToApplyFluxRBAC, err)
	}

	// Ensure FluxKubeconfig is created or updated
	fluxKubeconfig, err := r.ensureFluxKubeconfig(ctx, mcpCluster.RESTConfig(), tenantNamespace, v1beta1.ServiceAccountReference{
		Name:      "openmcp-flux-deployer",
		Namespace: "openmcp-system",
	})
	if err != nil {
		return ctrl.Result{}, errors.Join(errFailedToEnsureFluxKubeconfig, err)
	}
	ctx = rcontext.WithFluxKubeconfigRef(ctx, fluxKubeconfig)

	// Handle CreateOrUpdate
	//    1. Ensure finalizer is set on Crossplane instance AND ManagedControlPlane resource at Onboarding
	//    2. New Juggler

	conditions, err := r.createOrUpdateCrossplaneInstance(ctx, crossplane, mcpCluster.Client())
	if err != nil {
		log.Error(err, "failed to create or update Crossplane instance")
		return ctrl.Result{}, err
	}

	for _, c := range conditions {
		condApi.SetStatusCondition(&newConditions, c)
	}

	condApi.SetStatusCondition(&newConditions, v1beta1.Available())

	return ctrl.Result{}, nil
}

func (r *CrossplaneReconciler) createOrUpdateCrossplaneInstance(ctx context.Context, crossplane *v1alpha1.Crossplane, mcpClient client.Client) ([]metav1.Condition, error) {
	j, err := r.newJuggler(ctx, crossplane, mcpClient)
	if err != nil {
		return nil, err
	}
	result := j.Reconcile(ctx)

	enabledComponents := 0
	healthyComponents := 0
	conditions := []metav1.Condition{}
	for _, componentResult := range result {
		if componentResult.Component.IsEnabled() {
			enabledComponents++
		}
		if componentResult.Result == juggler.StatusHealthy {
			healthyComponents++
		}

		if !componentResult.Component.IsEnabled() && componentResult.Result == juggler.StatusDisabled {
			// Component is not enabled and has been successfully uninstalled (or has never been installed).
			// Don't output a condition in this case.
			continue
		}
		conditions = append(conditions, componentResult.ToCondition())
	}

	return conditions, nil
}

func (r *CrossplaneReconciler) newJuggler(ctx context.Context, xp *v1alpha1.Crossplane, mcpClient client.Client) (*juggler.Juggler, error) {
	logger := log.FromContext(ctx)
	jug := juggler.NewJuggler(logger, juggler.NewEventRecorder(r.Recorder, xp))

	xpComp := &component.Crossplane{
		Config: &xp.Spec,
	}
	jug.RegisterComponent(xpComp)

	r.registerReconcilers(jug, logger, mcpClient)

	if err := jug.RegisterOrphanedComponents(ctx); err != nil {
		return nil, err
	}

	return jug, nil
}

func (r *CrossplaneReconciler) registerReconcilers(juggler *juggler.Juggler, logger logr.Logger, mcpClient client.Client) {
	fr := fluxcd.NewFluxReconciler(logger, r.PlatformCluster.Client(), mcpClient)
	fr.RegisterType(
		&component.Crossplane{},
	)
	juggler.RegisterReconciler(fr)
}

// SetupWithManager sets up the controller with the Manager.
func (r *CrossplaneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.ClusterAccessReconciler = clusteraccess.NewClusterAccessReconciler(r.PlatformCluster.Client(), v1alpha1.GroupVersion.Group)
	r.ClusterAccessReconciler.
		WithMCPScheme(scheme.MCP).
		WithRetryInterval(10 * time.Second).
		WithMCPPermissions(getMCPPermissions())

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

func (r *CrossplaneReconciler) ensureFluxKubeconfig(ctx context.Context, mcpRestConfig *rest.Config, namespace string, saRef v1beta1.ServiceAccountReference) (*corev1.SecretReference, error) {
	if kubeconfigBuffer >= kubeconfigExpiration {
		return nil, errInvalidExpirationOrBuffer
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux-kubeconfig",
			Namespace: namespace,
		},
	}
	if err := r.PlatformCluster.Client().Get(ctx, client.ObjectKeyFromObject(secret), secret); client.IgnoreNotFound(err) != nil {
		return nil, err
	}

	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}

	if expirationStr, ok := secret.Data[keyExpiration]; ok {
		expiration, err := time.Parse(time.RFC3339, string(expirationStr))
		if err != nil {
			return nil, err
		}

		if time.Now().Before(expiration.Add(-kubeconfigBuffer)) {
			// kubeconfig is still valid
			return &corev1.SecretReference{Name: secret.Name, Namespace: secret.Namespace}, nil
		}
	}

	kubeconfig, expiration, err := r.Kubeconfiggen.ForServiceAccount(ctx, mcpRestConfig, saRef, kubeconfigExpiration)
	if err != nil {
		return nil, err
	}

	kubeconfigBytes, err := clientcmd.Write(*kubeconfig)
	if err != nil {
		return nil, err
	}

	_, err = controllerutil.CreateOrUpdate(ctx, r.PlatformCluster.Client(), secret, func() error {
		SetLabel(secret, "app.kubernetes.io/managed-by", "service-provider-crossplane")

		if secret.Data == nil {
			secret.Data = map[string][]byte{}
		}

		secret.Data[keyKubeconfig] = kubeconfigBytes
		secret.Data[keyExpiration] = []byte(expiration.Format(time.RFC3339))
		return nil
	})

	return &corev1.SecretReference{Name: secret.Name, Namespace: secret.Namespace}, err
}

// GetResolverFunc is used to verify if the Crossplane instance configuration with its providers is valid.
// It checks the name and versions against the configured v1alpha1.ProviderConfig on the Platform cluster.
// The function returns a v1beta1.VersionResolverFn that can be used to resolve the versions later in the reconcile loop.
func (r *CrossplaneReconciler) GetResolverFunc(providerConfig *v1alpha1.ProviderConfig) v1beta1.VersionResolverFn {
	return func(componentName string, version string) (v1beta1.ComponentVersion, error) {
		// Check if Crossplane is installable
		if componentName == component.CrossplaneRelease {
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

// SetLabel sets a label on the given object.
func SetLabel(obj metav1.Object, label string, value string) {
	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[label] = value
	obj.SetLabels(labels)
}
