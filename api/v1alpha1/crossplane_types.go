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
	crossplanev1 "github.com/crossplane/crossplane/apis/pkg/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CrossplaneProviderConfig represents configuration for Crossplane providers in a ControlPlane.
// Primarily based on the Crossplane open source API.
type CrossplaneProviderConfig struct {
	// Name of the provider.
	// Using a well-known name will automatically configure the "package" field.
	Name string `json:"name"`

	// Version of the provider to install.
	Version string `json:"version"`

	// Provider package to be installed.
	// If "name" is set to a well-known value, this field will be configured automatically.
	// +kubebuilder:validation:Optional
	Package string `json:"package,omitempty"`

	// Pull policy for the provider.
	// One of Always, Never, IfNotPresent.
	// +kubebuilder:default=IfNotPresent
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	PackagePullPolicy *corev1.PullPolicy `json:"packagePullPolicy,omitempty"`

	// PackagePullSecrets are named secrets in the same namespace that can be used to fetch packages from private registries.
	PackagePullSecrets []corev1.LocalObjectReference `json:"packagePullSecrets,omitempty"`

	crossplanev1.PackageRuntimeSpec `json:",inline"`
}

// CrossplaneSpec defines the desired state of Crossplane
type CrossplaneSpec struct {
	// The Version of Crossplane to install.
	Version string `json:"version"`

	// List of Crossplane providers to be installed.
	// +kubebuilder:validation:Optional
	Providers []*CrossplaneProviderConfig `json:"providers,omitempty"`
}

// CrossplaneStatus defines the observed state of Crossplane.
type CrossplaneStatus struct {
	// Current service state of the ProviderConfig.
	Conditions []metav1.Condition `json:"conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Crossplane is the Schema for the crossplanes API
type Crossplane struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of Crossplane
	// +required
	Spec CrossplaneSpec `json:"spec"`

	// status defines the observed state of Crossplane
	// +optional
	Status CrossplaneStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// CrossplaneList contains a list of Crossplane
type CrossplaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Crossplane `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Crossplane{}, &CrossplaneList{})
}
