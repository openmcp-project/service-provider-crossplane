/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"github.com/openmcp-project/extensibility-utils/pkg/discovery"
	commonapi "github.com/openmcp-project/openmcp-operator/api/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ocmv1alpha1 "ocm.software/open-component-model/bindings/go/kubernetes/controller/api/v1alpha1"
)

const (
	DiscoveryIDCrossplane = "crossplane"
	DiscoveryIDProviders  = "providers"
	DiscoveryIDFunctions  = "functions"
)

type CrossplaneConfig struct {
	// +kubebuilder:validation:Optional
	ResourceLabel string `json:"resourceLabel,omitempty"`
	// +kubebuilder:validation:Optional
	Chart string `json:"chart,omitempty"`
	// +kubebuilder:validation:Optional
	Image string `json:"image,omitempty"`
	// +kubebuilder:validation:Optional
	ComponentSelector *ocmv1alpha1.Selector `json:"componentSelector,omitempty"`
	// +kubebuilder:validation:Optional
	ResourceSelector *ocmv1alpha1.Selector `json:"resourceSelector,omitempty"`
}

type DiscoveredProvidersConfig struct {
	// +kubebuilder:validation:Optional
	ResourceLabel string `json:"resourceLabel,omitempty"`
	// +kubebuilder:validation:Optional
	Image string `json:"image,omitempty"`
	// +kubebuilder:validation:Optional
	ComponentSelector *ocmv1alpha1.Selector `json:"componentSelector,omitempty"`
	// +kubebuilder:validation:Optional
	ResourceSelector *ocmv1alpha1.Selector `json:"resourceSelector,omitempty"`
}

type DiscoveredFunctionsConfig struct {
	// +kubebuilder:validation:Optional
	ResourceLabel string `json:"resourceLabel,omitempty"`
	// +kubebuilder:validation:Optional
	Image string `json:"image,omitempty"`
	// +kubebuilder:validation:Optional
	ComponentSelector *ocmv1alpha1.Selector `json:"componentSelector,omitempty"`
	// +kubebuilder:validation:Optional
	ResourceSelector *ocmv1alpha1.Selector `json:"resourceSelector,omitempty"`
}

// ProviderConfigSpec defines the desired state of ProviderConfig.
type ProviderConfigSpec struct {
	// +kubebuilder:validation:Optional
	Crossplane CrossplaneConfig `json:"crossplane,omitempty"`
	// +kubebuilder:validation:Optional
	Providers DiscoveredProvidersConfig `json:"providers,omitempty"`
	// +kubebuilder:validation:Optional
	Functions DiscoveredFunctionsConfig `json:"functions,omitempty"`
	// +kubebuilder:validation:Optional
	Releasechannel string `json:"releasechannel,omitempty"`

	// CABundleRef is a reference to a config map containing certificate bundle.
	// It will be installed on the ManagedControlPlane and configured for Crossplane runtime and providers.
	// +kubebuilder:validation:Optional
	CABundleRef *corev1.ConfigMapKeySelector `json:"caBundleRef,omitempty"`
}

// ProviderConfigStatus defines the observed state of ProviderConfig.
type ProviderConfigStatus struct {
	commonapi.Status `json:",inline"`
}

// ProviderConfig is the Schema for the providerconfigs API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:metadata:labels="openmcp.cloud/cluster=platform"
type ProviderConfig struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of ProviderConfig
	// +required
	Spec ProviderConfigSpec `json:"spec"`

	// status defines the observed state of ProviderConfig
	// +optional
	Status ProviderConfigStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// ProviderConfigList contains a list of ProviderConfig
type ProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProviderConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &ProviderConfig{}, &ProviderConfigList{})
		return nil
	})
}

func (p *ProviderConfig) DiscoveryConfigs() []discovery.DiscoveryConfig {
	return []discovery.DiscoveryConfig{p.crossplaneDiscovery(), p.providersDiscovery(), p.functionsDiscovery()}
}

func (p *ProviderConfig) crossplaneDiscovery() discovery.DiscoveryConfig {
	componentSelector := p.Spec.Crossplane.ComponentSelector
	if componentSelector == nil {
		componentSelector = &ocmv1alpha1.Selector{
			MatchLabels: map[string]string{"crossplane.services.open-control-plane.io/component": "crossplane"},
		}
	}
	releasechannel := "releasechannel"
	if p.Spec.Releasechannel != "" {
		releasechannel = p.Spec.Releasechannel
	}

	return discovery.DiscoveryConfig{
		ID:                DiscoveryIDCrossplane,
		ComponentSelector: componentSelector,
		ResourceSelector:  p.Spec.Crossplane.ResourceSelector,
		Releasechannel:    releasechannel,
	}
}

func (p *ProviderConfig) providersDiscovery() discovery.DiscoveryConfig {
	componentSelector := p.Spec.Providers.ComponentSelector
	if componentSelector == nil {
		componentSelector = &ocmv1alpha1.Selector{
			MatchLabels: map[string]string{"crossplane.services.open-control-plane.io/component": "crossplane-provider"},
		}
	}
	releasechannel := "releasechannel"
	if p.Spec.Releasechannel != "" {
		releasechannel = p.Spec.Releasechannel
	}

	return discovery.DiscoveryConfig{
		ID:                DiscoveryIDProviders,
		ComponentSelector: componentSelector,
		ResourceSelector:  p.Spec.Providers.ResourceSelector,
		Releasechannel:    releasechannel,
	}
}

func (p *ProviderConfig) functionsDiscovery() discovery.DiscoveryConfig {
	componentSelector := p.Spec.Functions.ComponentSelector
	if componentSelector == nil {
		componentSelector = &ocmv1alpha1.Selector{
			MatchLabels: map[string]string{"crossplane.services.open-control-plane.io/component": "crossplane-function"},
		}
	}
	releasechannel := "releasechannel"
	if p.Spec.Releasechannel != "" {
		releasechannel = p.Spec.Releasechannel
	}

	return discovery.DiscoveryConfig{
		ID:                DiscoveryIDFunctions,
		ComponentSelector: componentSelector,
		ResourceSelector:  p.Spec.Functions.ResourceSelector,
		Releasechannel:    releasechannel,
	}
}
