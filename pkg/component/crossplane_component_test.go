//nolint:dupl
package component

import (
	"testing"

	"github.com/openmcp-project/control-plane-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/openmcp-project/service-provider-crossplane/api/v1alpha1"
)

func Test_Crossplane(t *testing.T) {
	testCases := []struct {
		desc                 string
		config               *v1alpha1.CrossplaneSpec
		configValues         *apiextensionsv1.JSON
		caBundleRef          *corev1.ConfigMapKeySelector
		imagePullSecretNames []string
		enabled              bool
		versionResolver      v1beta1.VersionResolverFn
		validationFuncs      []validationFunc
	}{
		{
			desc: "should be disabled",
			validationFuncs: []validationFunc{
				hasName("Crossplane"),
				isEnabled(false),
			},
		},
		{
			desc: "should not be allowed",
			config: &v1alpha1.CrossplaneSpec{
				Version: "1.2.3",
			},
			enabled:         true,
			versionResolver: fakeVersionResolver(true),
			validationFuncs: []validationFunc{
				hasName("Crossplane"),
				isEnabled(true),
				isAllowed(false),
			},
		},
		{
			desc: "should be enabled",
			config: &v1alpha1.CrossplaneSpec{
				Version: "1.2.3",
			},
			configValues:         &apiextensionsv1.JSON{Raw: []byte(`{"replicas":2}`)},
			imagePullSecretNames: []string{"secret-1", "secret-2"},
			enabled:              true,
			versionResolver:      fakeVersionResolver(false),
			validationFuncs: []validationFunc{
				hasName("Crossplane"),
				isEnabled(true),
				isAllowed(true),
				hasPreUninstallHook(),
				hasDependencies(0),
				isTargetComponent(
					hasNamespace(CrossplaneNamespace),
				),
				isFluxComponent(
					returnsOCIRepository(),
					returnsHelmRelease(
						hasKubeconfigRef(),
						hasHelmValue(2, "replicas"), // custom value
						hasHelmValue("secret-1", "imagePullSecrets", "0"),
						hasHelmValue("secret-2", "imagePullSecrets", "1"),
					),
				),
			},
		},
		{
			desc: "should be enabled with CA bundle",
			config: &v1alpha1.CrossplaneSpec{
				Version: "1.2.3",
			},
			configValues: &apiextensionsv1.JSON{Raw: []byte(`{"replicas":2}`)},
			caBundleRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "some-configmap-name",
				},
				Key: "ca.crt",
			},
			imagePullSecretNames: []string{"secret-1", "secret-2"},
			enabled:              true,
			versionResolver:      fakeVersionResolver(false),
			validationFuncs: []validationFunc{
				hasName("Crossplane"),
				isEnabled(true),
				isAllowed(true),
				hasPreUninstallHook(),
				hasDependencies(0),
				isTargetComponent(
					hasNamespace(CrossplaneNamespace),
				),
				isFluxComponent(
					returnsOCIRepository(),
					returnsHelmRelease(
						hasKubeconfigRef(),
						hasHelmValue(2, "replicas"), // custom value
						hasHelmValue("secret-1", "imagePullSecrets", "0"),
						hasHelmValue("secret-2", "imagePullSecrets", "1"),
						hasHelmValue("custom-ca-bundle", "registryCaBundleConfig", "name"),
						hasHelmValue("ca.crt", "registryCaBundleConfig", "key"),
					),
				),
			},
		},
		{
			desc: "should be enabled without image override when no image is specified",
			config: &v1alpha1.CrossplaneSpec{
				Version: "1.2.3",
			},
			configValues:         &apiextensionsv1.JSON{Raw: []byte(`{"replicas":2}`)},
			imagePullSecretNames: []string{"secret-1"},
			enabled:              true,
			versionResolver:      fakeVersionResolverNoDockerRef(),
			validationFuncs: []validationFunc{
				hasName("Crossplane"),
				isEnabled(true),
				isAllowed(true),
				isFluxComponent(
					returnsOCIRepository(),
					returnsHelmRelease(
						hasHelmValue(2, "replicas"), // custom value
						hasHelmValue("secret-1", "imagePullSecrets", "0"),
						lacksHelmKey("image"), // image key must not be set when no image URL provided
					),
				),
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			ctx := newContext(tC.versionResolver)
			c := &Crossplane{
				Config:               tC.config,
				Values:               tC.configValues,
				CABundleRef:          tC.caBundleRef,
				ChartPullSecretName:  "chart-pull-secret",
				ImagePullSecretNames: tC.imagePullSecretNames,
				Enabled:              tC.enabled,
			}
			for _, vfn := range tC.validationFuncs {
				vfn(t, ctx, c)
			}
		})
	}
}
