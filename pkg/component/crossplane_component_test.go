//nolint:dupl
package component

import (
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/openmcp-project/control-plane-operator/api/v1beta1"

	"github.com/openmcp-project/service-provider-crossplane/api/v1alpha1"
)

func Test_Crossplane(t *testing.T) {
	testCases := []struct {
		desc                 string
		config               *v1alpha1.CrossplaneSpec
		configValues         *apiextensionsv1.JSON
		imagePullSecretNames []string
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
					returnsHelmRepo(),
					returnsHelmRelease(
						hasKubeconfigRef(),
						hasHelmValue(2, "replicas"), // custom value
						hasHelmValue("secret-1", "imagePullSecrets", "0"),
						hasHelmValue("secret-2", "imagePullSecrets", "1"),
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
				ImagePullSecretNames: tC.imagePullSecretNames,
			}
			for _, vfn := range tC.validationFuncs {
				vfn(t, ctx, c)
			}
		})
	}
}
