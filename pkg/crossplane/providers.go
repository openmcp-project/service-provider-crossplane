package crossplane

import (
	crossplanev1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/openmcp-project/service-provider-crossplane/api/v1alpha1"
)

// EmptyFromConfig converts a CrossplaneProviderConfig to a Crossplane Provider resource.
func EmptyFromConfig(c v1alpha1.CrossplaneProviderConfig) (*crossplanev1.Provider, types.NamespacedName) {
	return &crossplanev1.Provider{}, types.NamespacedName{
		Name: ProviderNameForProviderConfig(&c),
	}
}

// ProviderNameForProviderConfig returns the name of a Provider crossplane manifest for a ProviderConfig.
// It consists of the name of the provider with a prefix.
func ProviderNameForProviderConfig(p *v1alpha1.CrossplaneProviderConfig) string {
	return p.Name
}
