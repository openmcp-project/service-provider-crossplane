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

package discovery

import (
	"sort"
	"strings"
	"sync"

	"github.com/Masterminds/semver/v3"
	"github.com/openmcp-project/control-plane-operator/api/v1beta1"
)

// Store holds the component versions discovered at runtime. It is written by the discovery
// reconciler (one entry per discovery source) and read by the version resolver. It is safe for
// concurrent use.
type Store struct {
	mu sync.RWMutex
	// versions maps short component name -> version string -> resolved ComponentVersion.
	versions map[string]map[string]v1beta1.ComponentVersion
	// pullSecrets maps short component name -> version string -> pull secret name (may be empty).
	pullSecrets map[string]map[string]string
}

// NewStore returns an empty Store.
func NewStore() *Store {
	return &Store{
		versions:    map[string]map[string]v1beta1.ComponentVersion{},
		pullSecrets: map[string]map[string]string{},
	}
}

// Set replaces all versions known for the given short component name. The pullSecrets map is keyed
// by version and may be nil when no pull secrets were discovered.
func (s *Store) Set(componentName string, versions map[string]v1beta1.ComponentVersion, pullSecrets map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(versions) == 0 {
		delete(s.versions, componentName)
		delete(s.pullSecrets, componentName)
		return
	}
	s.versions[componentName] = versions
	if len(pullSecrets) == 0 {
		delete(s.pullSecrets, componentName)
	} else {
		s.pullSecrets[componentName] = pullSecrets
	}
}

// Delete removes all versions known for the given short component name.
func (s *Store) Delete(componentName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.versions, componentName)
	delete(s.pullSecrets, componentName)
}

// Resolve returns the ComponentVersion for the given short component name and version.
// The second return value reports whether the version was found.
func (s *Store) Resolve(componentName, version string) (v1beta1.ComponentVersion, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	byVersion, ok := s.versions[componentName]
	if !ok {
		return v1beta1.ComponentVersion{}, false
	}
	comp, ok := byVersion[version]
	return comp, ok
}

// PullSecret returns the pull secret name for the given short component name and version, or the
// empty string if none was discovered.
func (s *Store) PullSecret(componentName, version string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pullSecrets[componentName][version]
}

// HasPullSecret reports whether the given secret name is referenced by any discovered component.
func (s *Store) HasPullSecret(name string) bool {
	if name == "" {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, byVersion := range s.pullSecrets {
		for _, secret := range byVersion {
			if secret == name {
				return true
			}
		}
	}
	return false
}

// AvailableVersions returns the versions known for the given short component name, sorted by
// semantic version (ascending). Versions that don't parse as semver fall back to lexical ordering
// after the valid ones, so the list is stable and human-readable in error messages.
func (s *Store) AvailableVersions(componentName string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	byVersion := s.versions[componentName]
	out := make([]string, 0, len(byVersion))
	for v := range byVersion {
		out = append(out, v)
	}
	sortVersions(out)
	return out
}

// sortVersions sorts version strings by semver, falling back to lexical order for unparseable ones.
func sortVersions(versions []string) {
	sort.Slice(versions, func(i, j int) bool {
		vi, ei := semver.NewVersion(versions[i])
		vj, ej := semver.NewVersion(versions[j])
		if ei != nil || ej != nil {
			if ei != nil && ej != nil {
				return versions[i] < versions[j]
			}
			return ej != nil // valid semver sorts before unparseable
		}
		return vi.LessThan(vj)
	})
}

// ShortName returns the short component name from a full component path, e.g.
// "github.com/openmcp-project/releasechannel/crossplane" -> "crossplane".
func ShortName(fullName string) string {
	if i := strings.LastIndex(fullName, "/"); i >= 0 {
		return fullName[i+1:]
	}
	return fullName
}

// BuildComponentVersions converts a list of discovered components into a map keyed by version.
// The helmChart resource populates OCIURL and the ociImage resource populates DockerRef; a
// component version that has both (Crossplane) yields a single ComponentVersion with both set.
// The imageReference is stored verbatim as it already carries the tag (and digest).
// It also returns the per-version pull secret names discovered alongside the components.
func BuildComponentVersions(components []Component) (map[string]v1beta1.ComponentVersion, map[string]string) {
	out := map[string]v1beta1.ComponentVersion{}
	pullSecrets := map[string]string{}
	for _, comp := range components {
		cv := out[comp.Version]
		cv.Version = comp.Version
		for _, res := range comp.Resources {
			switch res.Type {
			case TypeHelmChart:
				cv.OCIURL = res.Access.ImageReference
			case TypeOCIImage:
				cv.DockerRef = res.Access.ImageReference
			}
		}
		out[comp.Version] = cv
		if comp.PullSecret != "" {
			pullSecrets[comp.Version] = comp.PullSecret
		}
	}
	return out, pullSecrets
}
