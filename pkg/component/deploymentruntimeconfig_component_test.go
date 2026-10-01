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

func Test_setPollArg(t *testing.T) {
	testCases := []struct {
		desc          string
		drc           *crossplanev1beta1.DeploymentRuntimeConfig
		poll          metav1.Duration
		wantArgs      []string
		wantOtherKept bool // "other" container should still be present untouched
	}{
		{
			desc:     "creates template, spec and container when absent",
			drc:      &crossplanev1beta1.DeploymentRuntimeConfig{},
			poll:     metav1.Duration{Duration: 5 * time.Minute},
			wantArgs: []string{"--poll=5m0s"},
		},
		{
			desc:     "appends to an existing package-runtime container, preserving its args",
			drc:      drcWithContainer("package-runtime", "--debug"),
			poll:     metav1.Duration{Duration: 90 * time.Second},
			wantArgs: []string{"--debug", "--poll=1m30s"},
		},
		{
			desc:          "adds package-runtime alongside an unrelated container",
			drc:           drcWithContainer("sidecar", "--foo"),
			poll:          metav1.Duration{Duration: time.Hour},
			wantArgs:      []string{"--poll=1h0m0s"},
			wantOtherKept: true,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			setPollArg(tC.drc, tC.poll)

			assert.Equal(t, tC.wantArgs, packageRuntimeArgs(tC.drc))

			if tC.wantOtherKept {
				var found bool
				for _, c := range tC.drc.Spec.DeploymentTemplate.Spec.Template.Spec.Containers {
					if c.Name == "sidecar" {
						found = true
						assert.Equal(t, []string{"--foo"}, c.Args)
					}
				}
				assert.True(t, found, "unrelated container should be preserved")
			}
		})
	}
}

func Test_existingPollArg(t *testing.T) {
	testCases := []struct {
		desc    string
		drc     *crossplanev1beta1.DeploymentRuntimeConfig
		want    *metav1.Duration
		wantErr bool
	}{
		{
			desc: "nil when no deployment template",
			drc:  &crossplanev1beta1.DeploymentRuntimeConfig{},
			want: nil,
		},
		{
			desc: "nil when package-runtime has no --poll arg",
			drc:  drcWithContainer("package-runtime", "--debug"),
			want: nil,
		},
		{
			desc: "nil when --poll is on a different container",
			drc:  drcWithContainer("sidecar", "--poll=5m"),
			want: nil,
		},
		{
			desc: "returns parsed duration when present",
			drc:  drcWithContainer("package-runtime", "--debug", "--poll=90s"),
			want: &metav1.Duration{Duration: 90 * time.Second},
		},
		{
			desc:    "errors on malformed duration",
			drc:     drcWithContainer("package-runtime", "--poll=duration"),
			wantErr: true,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			got, err := existingPollArg(tC.drc)
			if tC.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tC.want, got)
		})
	}
}

func Test_DeploymentRuntimeConfig_PollInterval(t *testing.T) {
	testCases := []struct {
		desc            string
		pollInterval    *metav1.Duration
		obj             *crossplanev1beta1.DeploymentRuntimeConfig
		wantArgs        []string
		wantNilTemplate bool
	}{
		{
			desc:         "uses ProviderConfig value when live object has none",
			pollInterval: &metav1.Duration{Duration: 5 * time.Minute},
			obj:          &crossplanev1beta1.DeploymentRuntimeConfig{},
			wantArgs:     []string{"--poll=5m0s"},
		},
		{
			desc:         "manual value on live object takes precedence",
			pollInterval: &metav1.Duration{Duration: 5 * time.Minute},
			obj:          drcWithContainer("package-runtime", "--poll=10m"),
			wantArgs:     []string{"--poll=10m0s"},
		},
		{
			desc:            "no arg added when neither config nor live object set it",
			pollInterval:    nil,
			obj:             &crossplanev1beta1.DeploymentRuntimeConfig{},
			wantNilTemplate: true,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			c := &DeploymentRuntimeConfig{
				Config:       &crossplanev1beta1.DeploymentRuntimeConfigSpec{},
				PollInterval: tC.pollInterval,
			}
			require.NoError(t, c.ReconcileObject(context.Background(), tC.obj))

			if tC.wantNilTemplate {
				assert.Nil(t, tC.obj.Spec.DeploymentTemplate)
				return
			}
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

func drcWithContainer(name string, args ...string) *crossplanev1beta1.DeploymentRuntimeConfig {
	return &crossplanev1beta1.DeploymentRuntimeConfig{
		Spec: crossplanev1beta1.DeploymentRuntimeConfigSpec{
			DeploymentTemplate: &crossplanev1beta1.DeploymentTemplate{
				Spec: &appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{{Name: name, Args: args}},
						},
					},
				},
			},
		},
	}
}
