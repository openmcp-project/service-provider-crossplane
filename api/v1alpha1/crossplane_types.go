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
	"k8s.io/apimachinery/pkg/runtime"
)

// CrossplaneProviderConfig represents configuration for Crossplane providers in a Crossplane instance.
type CrossplaneProviderConfig struct {
	// Name of the provider.
	Name string `json:"name"`

	// Version of the provider to install.
	Version string `json:"version"`
}

// CrossplaneFunctionConfig represents configuration for Crossplane functions in a Crossplane instance.
type CrossplaneFunctionConfig struct {
	// Name of the function.
	Name string `json:"name"`

	// Version of the function to install.
	Version string `json:"version"`
}

// CrossplaneSpec defines the desired state of a Crossplane instance.
type CrossplaneSpec struct {
	// The Version of Crossplane to install.
	Version string `json:"version"`

	// List of Crossplane providers to be installed.
	// +kubebuilder:validation:Optional
	Providers []*CrossplaneProviderConfig `json:"providers,omitempty"`

	// List of Crossplane functions to be installed.
	// +kubebuilder:validation:Optional
	Functions []*CrossplaneFunctionConfig `json:"functions,omitempty"`
}

// CrossplaneStatus defines the observed state of a Crossplane instance.
type CrossplaneStatus struct {
	commonapi.Status `json:",inline"`
}

// Crossplane is the Schema for the crossplanes API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=`.spec.version`,name="Version",type=string
// +kubebuilder:printcolumn:JSONPath=`.status.phase`,name="Phase",type=string
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:metadata:labels="openmcp.cloud/cluster=onboarding"
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
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &Crossplane{}, &CrossplaneList{})
		return nil
	})
}
