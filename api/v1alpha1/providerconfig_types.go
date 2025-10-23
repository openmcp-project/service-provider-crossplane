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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AvailableCrossplaneProvider represents configuration for Crossplane providers in a ProviderConfig of the Service Provider Crossplane.
type AvailableCrossplaneProvider struct {
	// Name of the provider.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Version of the provider to install.
	// +kubebuilder:validation:Required
	Versions []string `json:"versions"`

	// Package is the package name of the provider.
	// +kubebuilder:validation:Required
	Package string `json:"package"`
}

type CrossplaneProviders struct {
	// AvailableProviders holds the list of providers that can be configured with the Service Provider Crossplane.
	// +kubebuilder:validation:Required
	AvailableProviders []AvailableCrossplaneProvider `json:"availableProviders"`

	// Image pull secrets for Crossplane pods
	// +kubebuilder:validation:Optional
	ImagePullSecrets []commonapi.LocalObjectReference `json:"imagePullSecrets,omitempty"`
}

// ChartSpec defines the location and access of a Helm chart.
type ChartSpec struct {
	// URL is a reference to an OCI artifact repository hosted on a remote container registry where the Helm chart is stored.
	// +kubebuilder:validation:Required
	URL string `json:"url"`

	// SecretRef references a secret containing credentials to access the OCI artifact repository.
	// +kubebuilder:validation:Optional
	SecretRef commonapi.LocalObjectReference `json:"secretRef,omitempty"`
}

// ImageSpec defines the location and access a container image.
type ImageSpec struct {
	// URL is a reference to the container image location.
	// +kubebuilder:validation:Required
	URL string `json:"url"`

	// SecretRef references a secret containing credentials to access the container image repository.
	// +kubebuilder:validation:Optional
	SecretRef commonapi.LocalObjectReference `json:"secretRef,omitempty"`
}

type CrossplaneVersion struct {
	// Version of Crossplane.
	// +kubebuilder:validation:Required
	Version string `json:"version"`

	// Chart holds the Helm chart information for this Crossplane version.
	// +kubebuilder:validation:Required
	Chart ChartSpec `json:"chart"`

	// Image holds the Crossplane controller image information for this Crossplane version.
	// +kubebuilder:validation:Required
	Image ImageSpec `json:"image"`
}

// ProviderConfigSpec defines the desired state of ProviderConfig.
type ProviderConfigSpec struct {
	CrossplaneVersions []CrossplaneVersion `json:"versions"`

	// Providers holds the configuration for Crossplane providers that can be installed via the Service Provider Crossplane.
	// +kubebuilder:validation:Optional
	Providers CrossplaneProviders `json:"providers,omitempty"`
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
	SchemeBuilder.Register(&ProviderConfig{}, &ProviderConfigList{})
}
