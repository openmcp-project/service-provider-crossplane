//nolint:dupl,lll
package component

import (
	"fmt"
	"testing"

	commonv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	crossplanev1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openmcp-project/control-plane-operator/api/v1beta1"
	"github.com/openmcp-project/control-plane-operator/pkg/juggler"

	"github.com/openmcp-project/service-provider-crossplane/api/v1alpha1"
)

var (
	functionInstallPending = &crossplanev1.Function{}
	functionHealthy        = &crossplanev1.Function{
		Status: crossplanev1.FunctionStatus{
			ConditionedStatus: commonv1.ConditionedStatus{
				Conditions: []commonv1.Condition{
					{
						Type:   crossplanev1.TypeInstalled,
						Status: corev1.ConditionTrue,
					},
					{
						Type:    crossplanev1.TypeHealthy,
						Status:  corev1.ConditionTrue,
						Reason:  "Healthy",
						Message: "Healthy",
					},
				},
			},
		},
	}
	functionA = &crossplanev1.Function{
		ObjectMeta: metav1.ObjectMeta{
			Name: "function-a",
		},
		Spec: crossplanev1.FunctionSpec{
			PackageSpec: crossplanev1.PackageSpec{
				Package: "xpkg.example.com/example/function-example:v1.0.0",
			},
		},
	}
	functionAWithOutPrefix = &crossplanev1.Function{
		ObjectMeta: metav1.ObjectMeta{
			Name: "a",
		},
		Spec: crossplanev1.FunctionSpec{
			PackageSpec: crossplanev1.PackageSpec{
				Package: "xpkg.example.com/example/function-example:v1.0.0",
			},
		},
	}
	functionB = &crossplanev1.Function{
		ObjectMeta: metav1.ObjectMeta{
			Name: "function-b",
		},
		Spec: crossplanev1.FunctionSpec{
			PackageSpec: crossplanev1.PackageSpec{
				Package: "xpkg.example.com/example/function-example:v1.0.0",
			},
		},
	}
)

func Test_formatFunctionName(t *testing.T) {
	testCases := []struct {
		functionName string
		expected     string
	}{
		{
			functionName: "function-patch-and-transform",
			expected:     "FunctionPatchAndTransform",
		},
		{
			functionName: "patch-and-transform",
			expected:     "PatchAndTransform",
		},
		{
			functionName: "a",
			expected:     "A",
		},
		{
			functionName: "function-abc-xyz",
			expected:     "FunctionAbcXyz",
		},
		{
			functionName: "abc-xyz",
			expected:     "AbcXyz",
		},
	}
	for _, tC := range testCases {
		tName := fmt.Sprintf("%s -> %s", tC.functionName, tC.expected)
		t.Run(tName, func(t *testing.T) {
			actual := formatFunctionName(tC.functionName)
			assert.Equal(t, tC.expected, actual)
		})
	}
}

func Test_CrossplaneFunction(t *testing.T) {
	testCases := []struct {
		desc            string
		enabled         bool
		config          *v1alpha1.CrossplaneFunctionConfig
		versionResolver v1beta1.VersionResolverFn
		validationFuncs []validationFunc
	}{
		{
			desc:    "should be disabled",
			enabled: false,
			validationFuncs: []validationFunc{
				isEnabled(false),
			},
		},
		{
			desc:    "should not be allowed",
			enabled: true,
			config: &v1alpha1.CrossplaneFunctionConfig{
				Name: "function-patch-and-transform",
			},
			versionResolver: fakeVersionResolver(true),
			validationFuncs: []validationFunc{
				hasName("FunctionPatchAndTransform"),
				isEnabled(true),
				isAllowed(false),
			},
		},
		{
			desc:    "should be allowed with prefix",
			enabled: true,
			config: &v1alpha1.CrossplaneFunctionConfig{
				Name: "function-patch-and-transform",
			},
			versionResolver: func(componentName string, channelName string) (v1beta1.ComponentVersion, error) {
				if componentName == "function-patch-and-transform" {
					return v1beta1.ComponentVersion{}, nil
				}
				return v1beta1.ComponentVersion{}, errFake
			},
			validationFuncs: []validationFunc{
				hasName("FunctionPatchAndTransform"),
				isEnabled(true),
				isAllowed(true),
			},
		},
		{
			desc:    "should be rejected without prefix",
			enabled: true,
			config: &v1alpha1.CrossplaneFunctionConfig{
				Name: "patch-and-transform",
			},
			versionResolver: func(componentName string, channelName string) (v1beta1.ComponentVersion, error) {
				if componentName == "function-patch-and-transform" {
					return v1beta1.ComponentVersion{}, nil
				}
				return v1beta1.ComponentVersion{}, errFake
			},
			validationFuncs: []validationFunc{
				hasName("PatchAndTransform"),
				isEnabled(true),
				isAllowed(false),
			},
		},
		{
			desc:    "should be enabled",
			enabled: true,
			config: &v1alpha1.CrossplaneFunctionConfig{
				Name: "function-patch-and-transform",
			},
			versionResolver: fakeVersionResolver(false),
			validationFuncs: []validationFunc{
				hasName("FunctionPatchAndTransform"),
				isEnabled(true),
				isAllowed(true),
				hasDependencies(1),
				isTargetComponent(
					hasNamespace("crossplane-system"),
				),
				isObjectComponent(
					objectIsType(&crossplanev1.Function{}),
					canCheckHealthiness(functionInstallPending, juggler.ResourceHealthiness{
						Healthy: false,
						Message: "Function installation is pending (). ",
					}),
					canCheckHealthiness(functionHealthy, juggler.ResourceHealthiness{
						Healthy: true,
						Message: "Healthy: Healthy",
					}),
					canBuildAndReconcile(nil),
					implementsOrphanedObjectsDetector(
						listTypeIs(&crossplanev1.FunctionList{}),
						hasFilterCriteria(2),
						canConvert(&crossplanev1.FunctionList{Items: []crossplanev1.Function{*functionA}}, 1),
						canCheckSame(
							&CrossplaneFunction{Config: &v1alpha1.CrossplaneFunctionConfig{Name: functionA.Name}},
							&CrossplaneFunction{Config: &v1alpha1.CrossplaneFunctionConfig{Name: functionA.Name}},
							true),
						canCheckSame(
							&CrossplaneFunction{Config: &v1alpha1.CrossplaneFunctionConfig{Name: functionA.Name}},
							&CrossplaneFunction{Config: &v1alpha1.CrossplaneFunctionConfig{Name: functionAWithOutPrefix.Name}},
							false),
						canCheckSame(
							&CrossplaneFunction{Config: &v1alpha1.CrossplaneFunctionConfig{Name: functionA.Name}},
							&CrossplaneFunction{Config: &v1alpha1.CrossplaneFunctionConfig{Name: functionB.Name}},
							false),
					),
				),
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			ctx := newContext(tC.versionResolver)
			c := &CrossplaneFunction{Config: tC.config, Enabled: tC.enabled}
			for _, vfn := range tC.validationFuncs {
				vfn(t, ctx, c)
			}
		})
	}
}
