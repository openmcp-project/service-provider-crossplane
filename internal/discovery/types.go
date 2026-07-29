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

// Package discovery holds the parsing and in-memory storage of the component versions that are
// discovered at runtime. Versions are published in Discovery resources (delivery.ocm.software/v1alpha1)
// owned by the OCM controllers.
package discovery

import "strings"

// Resource types for parsing the discovery payload.
const (
	// TypeHelmChart is the resource type for a Helm chart artifact.
	TypeHelmChart = "helmChart"
	// TypeOCIImage is the resource type for a container image artifact.
	TypeOCIImage = "ociImage"

	// ImageResourcePrefix is the naming convention prefix for container image resources.
	// Resources named "image-<name>" are container images; resources without this prefix
	// that share a component version with an "image-" counterpart are Helm charts.
	ImageResourcePrefix = "image-"
)

// Access describes how to obtain a discovered resource.
type Access struct {
	Type           string `json:"type,omitempty"`
	ImageReference string `json:"imageReference,omitempty"`
}

// Resource is a single artifact belonging to a discovered component version (Helm chart or image).
type Resource struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
	Type    string `json:"type,omitempty"`
	Access  Access `json:"access"`
}

// Component is a single discovered component version with its resources.
type Component struct {
	// Name is the full component path, e.g. "github.com/openmcp-project/releasechannel/crossplane".
	Name string `json:"name,omitempty"`
	// Version of the component (channel version).
	Version string `json:"version,omitempty"`
	// PullSecret is the name of a pull secret to use for this component's resources, if any.
	PullSecret string `json:"pullSecret,omitempty"`
	// Resources are the artifacts that make up this component version.
	Resources []Resource `json:"resources,omitempty"`
}

// Entry represents a single entry in the flat Discovery status.discovery list.
// Fields correspond to the discoveryFields configured on the Discovery resource.
type Entry struct {
	ComponentName    string `json:"componentName"`
	ComponentVersion string `json:"componentVersion"`
	ImageRef         string `json:"imageRef"`
	Name             string `json:"name"`
	Type             string `json:"type,omitempty"`
}

// InferType determines the resource type for a discovery entry. If the entry has an explicit
// type field, it is returned. Otherwise, resources whose name starts with "image-" are ociImage;
// all others default to ociImage as well unless hasImageCounterpart is true (meaning there is an
// "image-<name>" resource for the same component version), in which case the entry is a helmChart.
func (e *Entry) InferType(hasImageCounterpart bool) string {
	if e.Type != "" {
		return e.Type
	}
	if strings.HasPrefix(e.Name, ImageResourcePrefix) {
		return TypeOCIImage
	}
	if hasImageCounterpart {
		return TypeHelmChart
	}
	return TypeOCIImage
}

// ToComponents converts a flat list of discovery entries into the grouped Component structure
// expected by BuildComponentVersions. It infers resource types from naming conventions when
// the type field is absent.
func ToComponents(entries []Entry, pullSecret string) []Component {
	// Build a set of (componentVersion, "image-"+name) to detect helmChart counterparts.
	imageNames := map[string]struct{}{}
	for _, e := range entries {
		if strings.HasPrefix(e.Name, ImageResourcePrefix) {
			imageNames[e.ComponentVersion+"/"+strings.TrimPrefix(e.Name, ImageResourcePrefix)] = struct{}{}
		}
	}

	// Group entries by (componentName, componentVersion).
	type key struct{ name, version string }
	grouped := map[key]*Component{}
	order := []key{}

	for _, e := range entries {
		k := key{e.ComponentName, e.ComponentVersion}
		comp, ok := grouped[k]
		if !ok {
			comp = &Component{
				Name:       e.ComponentName,
				Version:    e.ComponentVersion,
				PullSecret: pullSecret,
			}
			grouped[k] = comp
			order = append(order, k)
		}

		_, hasCounterpart := imageNames[e.ComponentVersion+"/"+e.Name]
		resType := e.InferType(hasCounterpart)

		comp.Resources = append(comp.Resources, Resource{
			Name: e.Name,
			Type: resType,
			Access: Access{
				ImageReference: e.ImageRef,
			},
		})
	}

	result := make([]Component, 0, len(order))
	for _, k := range order {
		result = append(result, *grouped[k])
	}
	return result
}
