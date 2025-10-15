//nolint:lll
package component

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openmcp-project/control-plane-operator/api/v1beta1"
	"github.com/openmcp-project/control-plane-operator/pkg/controlplane/components"
	"github.com/openmcp-project/control-plane-operator/pkg/juggler"
	"github.com/openmcp-project/control-plane-operator/pkg/juggler/fluxcd"
	"github.com/openmcp-project/control-plane-operator/pkg/juggler/object"
	"github.com/openmcp-project/control-plane-operator/pkg/utils/rcontext"
)

var (
	tenantNamespace = "tenant-namespace"
	fluxSecretRef   = &meta.KubeConfigReference{
		SecretRef: meta.SecretKeyReference{
			Name: "some-secret",
			Key:  "kubeconfig",
		},
	}

	errFake          = errors.New("some error")
	errNoPath        = errors.New("at least one path element needs to be specified")
	errMapNil        = errors.New("map is nil")
	errValueNotFound = errors.New("value not found")
)

func fakeVersionResolver(shouldFail bool) v1beta1.VersionResolverFn {
	return func(componentName string, channelName string) (v1beta1.ComponentVersion, error) {
		if shouldFail {
			return v1beta1.ComponentVersion{}, errFake
		}
		return v1beta1.ComponentVersion{
			DockerRef: strings.ToLower(componentName),
			Version:   "v1.0.0",
		}, nil
	}
}

type validationFunc func(t *testing.T, ctx context.Context, c juggler.Component)
type targetValidationFunc func(t *testing.T, ctx context.Context, c components.TargetComponent)
type fluxValidationFunc func(t *testing.T, ctx context.Context, c fluxcd.FluxComponent)
type helmReleaseValidationFunc func(t *testing.T, ctx context.Context, h *fluxcd.HelmReleaseManifesto)
type objectValidationFunc func(t *testing.T, ctx context.Context, c object.ObjectComponent)
type orphanedObjectsDetectorValidationFunc func(t *testing.T, ctx context.Context, dc object.DetectorContext)

func hasName(expected string) validationFunc {
	return func(t *testing.T, ctx context.Context, c juggler.Component) {
		assert.Equal(t, expected, c.GetName(), "GetName does not match")
	}
}

func isEnabled(expected bool) validationFunc {
	return func(t *testing.T, ctx context.Context, c juggler.Component) {
		assert.Equal(t, expected, c.IsEnabled(), "IsEnabled does not match")
	}
}

func isAllowed(expected bool) validationFunc {
	return func(t *testing.T, ctx context.Context, c juggler.Component) {
		actual, _ := c.IsInstallable(ctx)
		assert.Equal(t, expected, actual, "IsInstallable does not match")
	}
}

func hasNoHooks() validationFunc {
	return func(t *testing.T, ctx context.Context, c juggler.Component) {
		assert.Equal(t, juggler.ComponentHooks{}, c.Hooks())
	}
}

func hasPreUninstallHook() validationFunc {
	return func(t *testing.T, ctx context.Context, c juggler.Component) {
		assert.NotNil(t, c.Hooks().PreUninstall, "PreUninstall hook is nil")
	}
}

func hasDependencies(count int) validationFunc {
	return func(t *testing.T, ctx context.Context, c juggler.Component) {
		assert.Len(t, c.GetDependencies(), count, "len(GetDependencies) count does not match")
	}
}

func newContext(fn v1beta1.VersionResolverFn) context.Context {
	ctx := context.Background()
	ctx = rcontext.WithTenantNamespace(ctx, tenantNamespace)
	ctx = rcontext.WithFluxKubeconfigRef(ctx, &corev1.SecretReference{Name: fluxSecretRef.SecretRef.Name})
	ctx = rcontext.WithVersionResolver(ctx, fn)
	return ctx
}

func isTargetComponent(additionalValidations ...targetValidationFunc) validationFunc {
	return func(t *testing.T, ctx context.Context, c juggler.Component) {
		tc, ok := c.(components.TargetComponent)
		if !assert.True(t, ok, "not a TargetComponent") {
			return
		}

		for _, v := range additionalValidations {
			v(t, ctx, tc)
		}
	}
}

func hasNamespace(namespace string) targetValidationFunc {
	return func(t *testing.T, ctx context.Context, c components.TargetComponent) {
		assert.Equal(t, namespace, c.GetNamespace(), "GetNamespace does not match")
	}
}

func isFluxComponent(additionalValidations ...fluxValidationFunc) validationFunc {
	return func(t *testing.T, ctx context.Context, c juggler.Component) {
		fc, ok := c.(fluxcd.FluxComponent)
		if !assert.True(t, ok, "not a FluxComponent") {
			return
		}

		for _, v := range additionalValidations {
			v(t, ctx, fc)
		}
	}
}

func returnsOCIRepository() fluxValidationFunc {
	return func(t *testing.T, ctx context.Context, c fluxcd.FluxComponent) {
		s, err := c.BuildSourceRepository(ctx)
		assert.NoError(t, err)

		h, ok := s.(*fluxcd.OCIRepositoryAdapter)
		if !assert.True(t, ok, "not a OCIRepositoryAdapter") {
			return
		}
		assert.NotNil(t, h.Source)
	}
}

func returnsHelmRelease(additionalValidations ...helmReleaseValidationFunc) fluxValidationFunc {
	return func(t *testing.T, ctx context.Context, c fluxcd.FluxComponent) {
		m, err := c.BuildManifesto(ctx)
		assert.NoError(t, err)

		h, ok := m.(*fluxcd.HelmReleaseManifesto)
		if !assert.True(t, ok, "not a HelmReleaseManifesto") {
			return
		}
		assert.NotNil(t, h.Manifest)

		for _, v := range additionalValidations {
			v(t, ctx, h)
		}
	}
}

func hasKubeconfigRef() helmReleaseValidationFunc {
	return func(t *testing.T, ctx context.Context, h *fluxcd.HelmReleaseManifesto) {
		assert.NotNil(t, h.Manifest.Spec.KubeConfig)
		assert.Equal(t, "kubeconfig", h.Manifest.Spec.KubeConfig.SecretRef.Key, "KubeConfig.SecretRef.Key does not match")
	}
}

func hasHelmValue(expected any, path ...string) helmReleaseValidationFunc {
	return func(t *testing.T, ctx context.Context, h *fluxcd.HelmReleaseManifesto) {
		if assert.NotNil(t, h.Manifest.Spec.Values, "values are nil") {
			actual, err := getNestedValue(h.Manifest.GetValues(), path...)
			assert.NoError(t, err)
			assert.EqualValues(t, expected, actual)
		}
	}
}

func isObjectComponent(additionalValidations ...objectValidationFunc) validationFunc {
	return func(t *testing.T, ctx context.Context, c juggler.Component) {
		oc, ok := c.(object.ObjectComponent)
		if !assert.True(t, ok, "not an ObjectComponent") {
			return
		}

		for _, v := range additionalValidations {
			v(t, ctx, oc)
		}
	}
}

func objectIsType(sample client.Object) objectValidationFunc {
	return func(t *testing.T, ctx context.Context, c object.ObjectComponent) {
		obj, _, err := c.BuildObjectToReconcile(ctx)
		if !assert.NoError(t, err) {
			return
		}

		assert.IsType(t, sample, obj)
	}
}

func implementsOrphanedObjectsDetector(additionalValidations ...orphanedObjectsDetectorValidationFunc) objectValidationFunc {
	return func(t *testing.T, ctx context.Context, c object.ObjectComponent) {
		ood, ok := c.(object.OrphanedObjectsDetector)
		if !assert.True(t, ok, "not a OrphanedObjectsDetector") {
			return
		}

		for _, v := range additionalValidations {
			v(t, ctx, ood.OrphanDetectorContext())
		}
	}
}

func listTypeIs(sample client.ObjectList) orphanedObjectsDetectorValidationFunc {
	return func(t *testing.T, ctx context.Context, dc object.DetectorContext) {
		assert.IsType(t, sample, dc.ListType)
	}
}

func hasFilterCriteria(count int) orphanedObjectsDetectorValidationFunc {
	return func(t *testing.T, ctx context.Context, dc object.DetectorContext) {
		assert.Len(t, dc.FilterCriteria, count)
	}
}

func canConvert(sample client.ObjectList, count int) orphanedObjectsDetectorValidationFunc {
	return func(t *testing.T, ctx context.Context, dc object.DetectorContext) {
		result := dc.ConvertFunc(sample)
		assert.Len(t, result, count)
	}
}

func canCheckSame(configured, detected juggler.Component, expected bool) orphanedObjectsDetectorValidationFunc {
	return func(t *testing.T, ctx context.Context, dc object.DetectorContext) {
		actual := dc.SameFunc(configured, detected)
		assert.Equal(t, expected, actual, "components %s and %s are not the same", configured.GetName(), detected.GetName())
	}
}

func canCheckHealthiness(sample client.Object, expected juggler.ResourceHealthiness) objectValidationFunc {
	return func(t *testing.T, ctx context.Context, c object.ObjectComponent) {
		actual := c.IsObjectHealthy(sample)
		assert.Equal(t, expected, actual)
	}
}

func canBuildAndReconcile(expectedErr error) objectValidationFunc {
	return func(t *testing.T, ctx context.Context, c object.ObjectComponent) {
		obj, _, err := c.BuildObjectToReconcile(ctx)
		if err != nil {
			assert.ErrorIs(t, err, expectedErr)
			return
		}

		err = c.ReconcileObject(ctx, obj)
		assert.Equal(t, expectedErr, err)
	}
}

// getNestedValue extracts nested values from maps or lists
func getNestedValue(m map[string]any, path ...string) (any, error) {
	if m == nil {
		return nil, errMapNil
	}
	if len(path) == 0 {
		return nil, errNoPath
	}
	current := any(m)
	for _, p := range path {
		switch c := current.(type) {
		case map[string]any:
			if val, ok := c[p]; ok {
				current = val
			} else {
				return nil, errValueNotFound
			}
		case []any:
			index, err := strconv.Atoi(p)
			if err != nil || index < 0 || index >= len(c) {
				return nil, errValueNotFound
			}
			current = c[index]
		default:
			return nil, errValueNotFound
		}
	}
	return current, nil
}
