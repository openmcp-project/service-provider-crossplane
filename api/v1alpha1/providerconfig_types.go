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

// ChartSpec identifies a Helm chart.
type ChartSpec struct {
	// URL is a reference to an OCI artifact repository hosted on a remote container registry.
	// +kubebuilder:validation:Required
	URL string `json:"url"`

	// AvailableVersions of the Helm chart.
	// +kubebuilder:validation:Required
	AvailableVersions []string `json:"availableVersions"`
}

// ProviderConfigSpec defines the desired state of ProviderConfig.
type ProviderConfigSpec struct {
	// Optional custom Helm chart configuration.
	// +kubebuilder:validation:Required
	Chart ChartSpec `json:"chart"`

	// ImageMapping holds the information about exchangable image locations in the Helm chart.
	// +kubebuilder:validation:Optional
	ImageMapping map[string]string `json:"imageMapping,omitempty"`

	// AvailableProviders holds the list of providers that can be configured with the Service Provider Crossplane.
	// +kubebuilder:validation:Required
	AvailableProviders []AvailableCrossplaneProvider `json:"availableProviders"`

	// Image pull secrets for Crossplane pods
	// +kubebuilder:validation:Optional
	ImagePullSecrets []commonapi.LocalObjectReference `json:"imagePullSecrets,omitempty"`
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
