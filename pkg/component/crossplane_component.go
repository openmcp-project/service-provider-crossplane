package component

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/openmcp-project/control-plane-operator/api/v1beta1"
	"github.com/openmcp-project/control-plane-operator/pkg/controlplane/components"
	"github.com/openmcp-project/control-plane-operator/pkg/juggler/fluxcd"
	"github.com/openmcp-project/controller-utils/pkg/image"

	"github.com/openmcp-project/service-provider-crossplane/api/v1alpha1"
	"github.com/openmcp-project/service-provider-crossplane/pkg/utils"

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
	Values               *apiextensionsv1.JSON `json:"values,omitempty"`
	ChartPullSecretName  string
	ImagePullSecretNames []string
	Enabled              bool
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

	comp, _ := rfn(CrossplaneRelease, c.Config.Version)

	url, tag, _, err := image.ParseImage(comp.OCIURL)
	if err != nil {
		return nil, err
	}
	ociURL := utils.AddOCIPrefix(url)

	var secretRef *meta.LocalObjectReference
	if c.ChartPullSecretName != "" {
		secretRef = &meta.LocalObjectReference{
			Name: c.ChartPullSecretName,
		}
	}

	repo := &sourcev1.OCIRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      strings.ToLower(ComponentNameCrossplane),
			Namespace: rcontext.TenantNamespace(ctx),
		},
		Spec: sourcev1.OCIRepositorySpec{
			URL: ociURL,
			Reference: &sourcev1.OCIRepositoryRef{
				Tag: tag,
			},
			Timeout:   &metav1.Duration{Duration: 1 * time.Minute},
			SecretRef: secretRef,
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
	rfn := rcontext.VersionResolver(ctx)
	if err := c.applyDefaultValues(rfn); err != nil {
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
	return c.Enabled
}

func (c *Crossplane) applyDefaultValues(rfn v1beta1.VersionResolverFn) error {
	if c.Config == nil {
		return nil
	}

	comp, _ := rfn(CrossplaneRelease, c.Config.Version)

	// Read user-provided values
	values := map[string]any{}
	if c.Values != nil {
		if err := json.Unmarshal(c.Values.Raw, &values); err != nil {
			return err
		}
	}

	// Add imagePullSecrets if provided in ProviderConfig spec
	values["imagePullSecrets"] = c.ImagePullSecretNames

	url, tag, _, err := image.ParseImage(comp.DockerRef)
	if err != nil {
		return err
	}

	// Pull Deployment image from specified chart URL provided in ProviderConfig spec
	values["image"] = map[string]any{
		"repository": url,
		"tag":        tag,
	}

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
