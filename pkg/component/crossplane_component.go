package component

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/openmcp-project/control-plane-operator/api/v1beta1"
	"github.com/openmcp-project/control-plane-operator/pkg/controlplane/components"
	"github.com/openmcp-project/control-plane-operator/pkg/juggler/fluxcd"

	"github.com/openmcp-project/service-provider-crossplane/api/v1alpha1"

	"github.com/openmcp-project/control-plane-operator/pkg/juggler"
	"github.com/openmcp-project/control-plane-operator/pkg/juggler/hooks"
	"github.com/openmcp-project/control-plane-operator/pkg/utils/rcontext"
)

const (
	// CrossplaneRelease is the name of the Helm release for Crossplane.
	CrossplaneRelease = "crossplane"

	// CrossplaneNamespace is the namespace where Crossplane is installed.
	CrossplaneNamespace = "crossplane-system"

	// ComponentNameCrossplane is the name of the Crossplane component.
	ComponentNameCrossplane = "Crossplane"
)

var _ fluxcd.FluxComponent = &Crossplane{}
var _ components.TargetComponent = &Crossplane{}

// Crossplane represents the Crossplane component configuration.
type Crossplane struct {
	Config               *v1alpha1.CrossplaneSpec
	ChartSpec            *v1beta1.ChartSpec
	Values               *apiextensionsv1.JSON `json:"values,omitempty"`
	ImagePullSecretNames []string
}

// GetNamespace implements TargetComponent.
func (c *Crossplane) GetNamespace() string {
	return CrossplaneNamespace
}

// IsInstallable implements FluxComponent.
func (c *Crossplane) IsInstallable(ctx context.Context) (bool, error) {
	rfn := rcontext.VersionResolver(ctx)
	if _, err := rfn(CrossplaneRelease, c.Config.Version); err != nil {
		return false, err
	}
	return true, nil
}

// BuildSourceRepository implements FluxComponent.
func (c *Crossplane) BuildSourceRepository(ctx context.Context) (fluxcd.SourceAdapter, error) {
	rfn := rcontext.VersionResolver(ctx)
	c.applyDefaultChartSpec(rfn)

	repo := &sourcev1.OCIRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      strings.ToLower(ComponentNameCrossplane),
			Namespace: rcontext.TenantNamespace(ctx),
		},
		Spec: sourcev1.OCIRepositorySpec{
			URL: c.ChartSpec.URL,
			Reference: &sourcev1.OCIRepositoryRef{
				Tag: c.ChartSpec.Version,
			},
			Timeout: &metav1.Duration{Duration: 1 * time.Minute},
		},
	}

	adapter := &fluxcd.OCIRepositoryAdapter{Source: repo}
	adapter.ApplyDefaults()
	return adapter, nil
}

// BuildManifesto implements FluxComponent.
//
//nolint:dupl
func (c *Crossplane) BuildManifesto(ctx context.Context) (fluxcd.Manifesto, error) {
	if err := c.applyDefaultValues(); err != nil {
		return nil, err
	}

	release := &helmv2.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      strings.ToLower(ComponentNameCrossplane),
			Namespace: rcontext.TenantNamespace(ctx),
		},
		Spec: helmv2.HelmReleaseSpec{
			ChartRef: &helmv2.CrossNamespaceSourceReference{
				Kind: "OCIRepository",
				Name: strings.ToLower(ComponentNameCrossplane),
			},
			ReleaseName:      CrossplaneRelease,
			TargetNamespace:  CrossplaneNamespace,
			StorageNamespace: CrossplaneNamespace,
			KubeConfig:       rcontext.FluxKubeconfigRef(ctx),
			Values:           c.Values,
		},
	}

	adapter := &fluxcd.HelmReleaseManifesto{Manifest: release}
	adapter.ApplyDefaults()
	return adapter, nil
}

// GetName implements Component.
func (*Crossplane) GetName() string {
	return ComponentNameCrossplane
}

// GetDependencies implements Component.
func (*Crossplane) GetDependencies() []juggler.Component {
	// No dependencies
	return []juggler.Component{}
}

// IsEnabled implements Component.
func (c *Crossplane) IsEnabled() bool {
	return c.Config != nil && c.Config.Version != ""
}

func (c *Crossplane) applyDefaultChartSpec(rfn v1beta1.VersionResolverFn) {
	if c.Config == nil {
		c.Config = &v1alpha1.CrossplaneSpec{}
	}

	comp, _ := rfn(CrossplaneRelease, c.Config.Version)

	if c.ChartSpec == nil {
		c.ChartSpec = &v1beta1.ChartSpec{
			URL:     comp.OCIURL,
			Version: comp.Version,
		}
	}
}

func (c *Crossplane) applyDefaultValues() error {
	if c.Config == nil {
		return nil
	}

	// Read user-provided values
	values := map[string]any{}
	if c.Values != nil {
		if err := json.Unmarshal(c.Values.Raw, &values); err != nil {
			return err
		}
	}

	// Add imagePullSecrets if provided in ProviderConfig spec
	values["imagePullSecrets"] = c.ImagePullSecretNames

	// Write updated values
	encoded, err := json.Marshal(values)
	c.Values = &apiextensionsv1.JSON{Raw: encoded}
	return err
}

// Hooks implements Component.
func (*Crossplane) Hooks() juggler.ComponentHooks {
	return juggler.ComponentHooks{
		PreUninstall: hooks.PreventOrphanedResources([]schema.GroupVersionKind{
			{Group: "pkg.crossplane.io", Version: "v1", Kind: "Provider"},
			{Group: "pkg.crossplane.io", Version: "v1", Kind: "ProviderRevision"},
		}),
	}
}
