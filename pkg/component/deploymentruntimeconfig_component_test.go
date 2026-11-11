//nolint:dupl,lll
package component

import (
	"context"
	"testing"
	"time"

	crossplanev1beta1 "github.com/crossplane/crossplane/apis/v2/pkg/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
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
							&DeploymentRuntimeConfig{
								Name:   deploymentRuntimeConfigA.Name,
								Config: &crossplanev1beta1.DeploymentRuntimeConfigSpec{},
							},
							&DeploymentRuntimeConfig{
								Name:   deploymentRuntimeConfigA.Name,
								Config: &crossplanev1beta1.DeploymentRuntimeConfigSpec{},
							},
							true),
						canCheckSame(
							&DeploymentRuntimeConfig{
								Name:   deploymentRuntimeConfigA.Name,
								Config: &crossplanev1beta1.DeploymentRuntimeConfigSpec{},
							},
							&DeploymentRuntimeConfig{
								Name:   deploymentRuntimeConfigB.Name,
								Config: &crossplanev1beta1.DeploymentRuntimeConfigSpec{},
							},
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
							&DeploymentRuntimeConfig{
								Name:   deploymentRuntimeConfigA.Name,
								Config: &crossplanev1beta1.DeploymentRuntimeConfigSpec{},
							},
							&DeploymentRuntimeConfig{
								Name:   deploymentRuntimeConfigA.Name,
								Config: &crossplanev1beta1.DeploymentRuntimeConfigSpec{},
							},
							true),
						canCheckSame(
							&DeploymentRuntimeConfig{
								Name:   deploymentRuntimeConfigA.Name,
								Config: &crossplanev1beta1.DeploymentRuntimeConfigSpec{},
							},
							&DeploymentRuntimeConfig{
								Name:   deploymentRuntimeConfigB.Name,
								Config: &crossplanev1beta1.DeploymentRuntimeConfigSpec{},
							},
							false),
					),
				),
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			ctx := newContext(fakeVersionResolver(false))
			c := &DeploymentRuntimeConfig{Name: tC.config.Name, Config: &tC.config.Spec, Enabled: tC.enabled}
			for _, vfn := range tC.validationFuncs {
				vfn(t, ctx, c)
			}
		})
	}
}

func Test_getArgs(t *testing.T) {
	testCases := []struct {
		desc     string
		drc      *crossplanev1beta1.DeploymentRuntimeConfig
		wantArgs []string
	}{
		{
			desc:     "package-runtime container returns args",
			drc:      testDRC("1", []v1.Container{{Name: "package-runtime", Args: []string{"--arg=one", "--arg=two"}}}),
			wantArgs: []string{"--arg=one", "--arg=two"},
		},
		{
			desc:     "no package-runtime container returns no args",
			drc:      testDRC("1", []v1.Container{{Name: "not-package-runtime", Args: []string{"--arg=one", "--arg=two"}}}),
			wantArgs: nil,
		},
		{
			desc:     "multiple containers only returns package-runtime containers args",
			drc:      testDRC("1", []v1.Container{{Name: "not-package-runtime", Args: []string{"--arg=one", "--arg=two"}}, {Name: "package-runtime", Args: []string{"--arg=one", "--arg=two"}}}),
			wantArgs: []string{"--arg=one", "--arg=two"},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			args := getArgs(tC.drc)
			assert.Equal(t, tC.wantArgs, args)
		})
	}
}

func Test_setArgs(t *testing.T) {
	testCases := []struct {
		desc          string
		drc           *crossplanev1beta1.DeploymentRuntimeConfig
		args          []string
		wantArgs      []string
		wantOtherKept bool
	}{
		{
			desc:     "creates template, spec and container when absent",
			drc:      &crossplanev1beta1.DeploymentRuntimeConfig{},
			args:     []string{"--poll=5m0s"},
			wantArgs: []string{"--poll=5m0s"},
		},
		{
			desc:     "overrides existing package-runtime args",
			drc:      testDRC("1", []v1.Container{{Name: "package-runtime", Args: []string{"--arg=one", "--arg=two"}}}),
			args:     []string{"--poll=5m0s"},
			wantArgs: []string{"--poll=5m0s"},
		},
		{
			desc:          "only changes package-runtime",
			drc:           testDRC("1", []v1.Container{{Name: "not-package-runtime", Args: []string{"--arg=one", "--arg=two"}}, {Name: "package-runtime", Args: []string{"--arg=one", "--arg=two"}}}),
			args:          []string{"--poll=5m0s"},
			wantArgs:      []string{"--poll=5m0s"},
			wantOtherKept: true,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			setArgs(&tC.drc.Spec, tC.args)

			assert.Equal(t, tC.wantArgs, packageRuntimeArgs(tC.drc))

			if tC.wantOtherKept {
				var found bool
				for _, c := range tC.drc.Spec.DeploymentTemplate.Spec.Template.Spec.Containers {
					if c.Name == "not-package-runtime" {
						found = true
						assert.Equal(t, []string{"--arg=one", "--arg=two"}, c.Args)
					}
				}
				assert.True(t, found, "unrelated container should be preserved")
			}
		})
	}
}

func Test_DeploymentRuntimeConfig_PollInterval(t *testing.T) {
	testCases := []struct {
		desc         string
		pollInterval *metav1.Duration
		obj          *crossplanev1beta1.DeploymentRuntimeConfig
		wantArgs     []string
	}{
		{
			desc:         "adds no poll interval when not set",
			pollInterval: nil,
			obj:          testDRC("", []v1.Container{{Name: "package-runtime"}}),
			wantArgs:     nil,
		},
		{
			desc:         "adds poll interval when set and drc does not exist",
			pollInterval: &metav1.Duration{Duration: 5 * time.Minute},
			obj:          testDRC("", []v1.Container{{Name: "package-runtime"}}),
			wantArgs:     []string{"--poll=5m0s"},
		},
		{
			desc:         "does not overwrite when drc already existed",
			pollInterval: &metav1.Duration{Duration: 5 * time.Minute},
			obj:          testDRC("1", []v1.Container{{Name: "package-runtime", Args: []string{"--poll=1m0s"}}}),
			wantArgs:     []string{"--poll=1m0s"},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			c := &DeploymentRuntimeConfig{
				Config:       &crossplanev1beta1.DeploymentRuntimeConfigSpec{},
				PollInterval: tC.pollInterval,
			}
			require.NoError(t, c.ReconcileObject(context.Background(), tC.obj))

			assert.Equal(t, tC.wantArgs, packageRuntimeArgs(tC.obj))
		})
	}
}

func packageRuntimeArgs(drc *crossplanev1beta1.DeploymentRuntimeConfig) []string {
	dt := drc.Spec.DeploymentTemplate
	if dt == nil || dt.Spec == nil {
		return nil
	}
	for _, c := range dt.Spec.Template.Spec.Containers {
		if c.Name == "package-runtime" {
			return c.Args
		}
	}
	return nil
}

func testDRC(resourceVersion string, containers []v1.Container) *crossplanev1beta1.DeploymentRuntimeConfig {
	return &crossplanev1beta1.DeploymentRuntimeConfig{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: resourceVersion,
		},
		Spec: crossplanev1beta1.DeploymentRuntimeConfigSpec{
			DeploymentTemplate: &crossplanev1beta1.DeploymentTemplate{
				Spec: &appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: containers,
						},
					},
				},
			},
		},
	}
}
