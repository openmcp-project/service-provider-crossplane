package crossplane

import (
	"testing"

	crossplanev1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"

	"github.com/stretchr/testify/assert"

	"github.com/openmcp-project/service-provider-crossplane/api/v1alpha1"
)

func TestProviderNameForProviderConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   v1alpha1.CrossplaneProviderConfig
		expected string
	}{
		{
			name: "provider config with hyphenated name",
			config: v1alpha1.CrossplaneProviderConfig{
				Name: "provider-kubernetes",
			},
			expected: "provider-kubernetes",
		},
		{
			name: "empty name",
			config: v1alpha1.CrossplaneProviderConfig{
				Name: "",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProviderNameForProviderConfig(&tt.config)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEmptyFromConfig(t *testing.T) {
	tests := []struct {
		name         string
		config       v1alpha1.CrossplaneProviderConfig
		expectedName string
	}{
		{
			name: "basic provider config",
			config: v1alpha1.CrossplaneProviderConfig{
				Name:    "provider-kubernetes",
				Version: "v0.8.0",
			},
			expectedName: "provider-kubernetes",
		},
		{
			name: "empty name provider config",
			config: v1alpha1.CrossplaneProviderConfig{
				Name:    "",
				Version: "v1.0.0",
			},
			expectedName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, namespacedName := EmptyFromConfig(tt.config)

			// Verify the provider is not nil and is of correct type
			assert.NotNil(t, provider)

			// Verify it's an empty provider (no fields set except type metadata)
			expectedProvider := &crossplanev1.Provider{}
			assert.Equal(t, expectedProvider.ObjectMeta, provider.ObjectMeta)
			assert.Equal(t, expectedProvider.Spec, provider.Spec)
			assert.Equal(t, expectedProvider.Status, provider.Status)

			// Verify the namespaced name
			assert.Equal(t, tt.expectedName, namespacedName.Name)
			assert.Empty(t, namespacedName.Namespace) // Should be empty as per the function implementation
		})
	}
}
