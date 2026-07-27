package crossplane

import (
	crossplanev1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/openmcp-project/service-provider-crossplane/api/v1alpha1"
)

// EmptyFunctionFromConfig converts a CrossplaneFunctionConfig to a Crossplane Function resource.
func EmptyFunctionFromConfig(c v1alpha1.CrossplaneFunctionConfig) (*crossplanev1.Function, types.NamespacedName) {
	return &crossplanev1.Function{}, types.NamespacedName{
		Name: FunctionNameForFunctionConfig(&c),
	}
}

// FunctionNameForFunctionConfig returns the name of a Function crossplane manifest for a config.
func FunctionNameForFunctionConfig(f *v1alpha1.CrossplaneFunctionConfig) string {
	return f.Name
}
