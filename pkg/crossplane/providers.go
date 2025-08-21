package crossplane

import (
	"fmt"
	"strings"

	crossplanev1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/openmcp-project/service-provider-crossplane/api/v1alpha1"
)

const (
	providerPrefix = "provider-"
)

// EmptyFromConfig converts a CrossplaneProviderConfig to a Crossplane Provider resource.
func EmptyFromConfig(c v1alpha1.CrossplaneProviderConfig) (*crossplanev1.Provider, types.NamespacedName) {
	return &crossplanev1.Provider{}, types.NamespacedName{
		Name: ProviderNameForProviderConfig(&c),
	}
}

// AddProviderPrefix adds the provider prefix to the provider name if it is not already present.
func AddProviderPrefix(providerName string) string {
	if strings.HasPrefix(providerName, providerPrefix) {
		return providerName
	}
	return fmt.Sprintf("%s%s", providerPrefix, providerName)
}

// ProviderNameForProviderConfig returns the name of a Provider crossplane manifest for a ProviderConfig.
// It consists of the name of the provider with a prefix.
func ProviderNameForProviderConfig(p *v1alpha1.CrossplaneProviderConfig) string {
	return p.Name
}
