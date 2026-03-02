//nolint:dupl,lll
package component

import (
	"testing"

	crossplanev1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openmcp-project/control-plane-operator/pkg/juggler"
)

var (
	deploymentRuntimeConfigHealthy = &crossplanev1beta1.DeploymentRuntimeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
		Spec: crossplanev1beta1.DeploymentRuntimeConfigSpec{},
	}
	deploymentRuntimeConfigA = &crossplanev1beta1.DeploymentRuntimeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "config-a",
		},
		Spec: crossplanev1beta1.DeploymentRuntimeConfigSpec{},
	}
	deploymentRuntimeConfigB = &crossplanev1beta1.DeploymentRuntimeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "config-b",
		},
		Spec: crossplanev1beta1.DeploymentRuntimeConfigSpec{},
	}
)

func Test_DeploymentRuntimeConfig(t *testing.T) {
	testCases := []struct {
		desc            string
		enabled         bool
		config          *crossplanev1beta1.DeploymentRuntimeConfig
		validationFuncs []validationFunc
	}{
		{
			desc:    "should be disabled",
			enabled: false,
			config: &crossplanev1beta1.DeploymentRuntimeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
			},
			validationFuncs: []validationFunc{
				isEnabled(false),
			},
		},
		{
			desc:    "should be enabled with default name",
			enabled: true,
			config: &crossplanev1beta1.DeploymentRuntimeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
			},
			validationFuncs: []validationFunc{
				hasName("DeploymentRuntimeConfigDefault"),
				isEnabled(true),
				isAllowed(true),
				hasDependencies(1),
				isTargetComponent(
					hasNamespace("crossplane-system"),
				),
				isObjectComponent(
					objectIsType(&crossplanev1beta1.DeploymentRuntimeConfig{}),
					canCheckHealthiness(deploymentRuntimeConfigHealthy, juggler.ResourceHealthiness{
						Healthy: true,
						Message: "DeploymentRuntimeConfig is configured",
					}),
					canBuildAndReconcile(nil),
					implementsOrphanedObjectsDetector(
						listTypeIs(&crossplanev1beta1.DeploymentRuntimeConfigList{}),
						hasFilterCriteria(2),
						canConvert(&crossplanev1beta1.DeploymentRuntimeConfigList{Items: []crossplanev1beta1.DeploymentRuntimeConfig{*deploymentRuntimeConfigA}}, 1),
						canCheckSame(
							&DeploymentRuntimeConfig{Config: &crossplanev1beta1.DeploymentRuntimeConfig{
								ObjectMeta: metav1.ObjectMeta{Name: deploymentRuntimeConfigA.Name},
							}},
							&DeploymentRuntimeConfig{Config: &crossplanev1beta1.DeploymentRuntimeConfig{
								ObjectMeta: metav1.ObjectMeta{Name: deploymentRuntimeConfigA.Name},
							}},
							true),
						canCheckSame(
							&DeploymentRuntimeConfig{Config: &crossplanev1beta1.DeploymentRuntimeConfig{
								ObjectMeta: metav1.ObjectMeta{Name: deploymentRuntimeConfigA.Name},
							}},
							&DeploymentRuntimeConfig{Config: &crossplanev1beta1.DeploymentRuntimeConfig{
								ObjectMeta: metav1.ObjectMeta{Name: deploymentRuntimeConfigB.Name},
							}},
							false),
					),
				),
			},
		},
		{
			desc:    "should be enabled with custom name",
			enabled: true,
			config: &crossplanev1beta1.DeploymentRuntimeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "custom-config",
				},
			},
			validationFuncs: []validationFunc{
				hasName("DeploymentRuntimeConfigCustom-Config"),
				isEnabled(true),
				isAllowed(true),
				hasDependencies(1),
				isTargetComponent(
					hasNamespace("crossplane-system"),
				),
				isObjectComponent(
					objectIsType(&crossplanev1beta1.DeploymentRuntimeConfig{}),
					canCheckHealthiness(deploymentRuntimeConfigHealthy, juggler.ResourceHealthiness{
						Healthy: true,
						Message: "DeploymentRuntimeConfig is configured",
					}),
					canBuildAndReconcile(nil),
					implementsOrphanedObjectsDetector(
						listTypeIs(&crossplanev1beta1.DeploymentRuntimeConfigList{}),
						hasFilterCriteria(2),
						canConvert(&crossplanev1beta1.DeploymentRuntimeConfigList{Items: []crossplanev1beta1.DeploymentRuntimeConfig{*deploymentRuntimeConfigA}}, 1),
						canCheckSame(
							&DeploymentRuntimeConfig{Config: &crossplanev1beta1.DeploymentRuntimeConfig{
								ObjectMeta: metav1.ObjectMeta{Name: deploymentRuntimeConfigA.Name},
							}},
							&DeploymentRuntimeConfig{Config: &crossplanev1beta1.DeploymentRuntimeConfig{
								ObjectMeta: metav1.ObjectMeta{Name: deploymentRuntimeConfigA.Name},
							}},
							true),
						canCheckSame(
							&DeploymentRuntimeConfig{Config: &crossplanev1beta1.DeploymentRuntimeConfig{
								ObjectMeta: metav1.ObjectMeta{Name: deploymentRuntimeConfigA.Name},
							}},
							&DeploymentRuntimeConfig{Config: &crossplanev1beta1.DeploymentRuntimeConfig{
								ObjectMeta: metav1.ObjectMeta{Name: deploymentRuntimeConfigB.Name},
							}},
							false),
					),
				),
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			ctx := newContext(fakeVersionResolver(false))
			c := &DeploymentRuntimeConfig{Config: tC.config, Enabled: tC.enabled}
			for _, vfn := range tC.validationFuncs {
				vfn(t, ctx, c)
			}
		})
	}
}
