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
	commonapi "github.com/openmcp-project/openmcp-operator/api/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// CrossplaneProviders represents configuration shared by all Crossplane providers.
type CrossplaneProviders struct {
	// Image pull secrets for pulling Crossplane provider images from private OCI registries.
	// +kubebuilder:validation:Optional
	ImagePullSecrets []commonapi.LocalObjectReference `json:"imagePullSecretRefs,omitempty"`
}

// ProviderConfigSpec defines the desired state of ProviderConfig.
type ProviderConfigSpec struct {
	// CrossplaneDiscoveryName is the name of the Discovery resource (delivery.ocm.software/v1alpha1)
	// on the platform cluster that publishes the available Crossplane versions.
	// +kubebuilder:validation:Required
	CrossplaneDiscoveryName string `json:"crossplaneDiscoveryName"`

	// ProviderDiscoverySelector selects the Discovery resources (delivery.ocm.software/v1alpha1)
	// on the platform cluster that publish the available Crossplane provider versions.
	// +kubebuilder:validation:Required
	ProviderDiscoverySelector metav1.LabelSelector `json:"providerDiscoverySelector"`

	// Providers holds configuration shared by all Crossplane providers installed via the Service Provider Crossplane.
	// +kubebuilder:validation:Optional
	Providers CrossplaneProviders `json:"providers,omitempty"`

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
