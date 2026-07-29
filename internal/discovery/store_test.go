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
	"sync"
	"testing"

	"github.com/openmcp-project/control-plane-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
)

func TestShortName(t *testing.T) {
	tests := map[string]string{
		"github.com/openmcp-project/releasechannel/crossplane":          "crossplane",
		"github.com/openmcp-project/releasechannel/provider-kubernetes": "provider-kubernetes",
		"crossplane": "crossplane",
		"":           "",
	}
	for in, want := range tests {
		assert.Equal(t, want, ShortName(in), in)
	}
}

func TestBuildComponentVersions_Crossplane(t *testing.T) {
	// A crossplane component version has both a helmChart and an ociImage resource.
	components := []Component{
		{
			Name:       "github.com/openmcp-project/releasechannel/crossplane",
			Version:    "v2.0.8",
			PullSecret: "xp-pull",
			Resources: []Resource{
				{Name: "crossplane", Type: TypeHelmChart, Access: Access{ImageReference: "registry/charts/crossplane:2.0.8@sha256:aaa"}},
				{Name: "image-crossplane", Type: TypeOCIImage, Access: Access{ImageReference: "registry/images/crossplane:v2.0.8@sha256:bbb"}},
			},
		},
	}

	versions, pullSecrets := BuildComponentVersions(components)

	assert.Len(t, versions, 1)
	cv := versions["v2.0.8"]
	assert.Equal(t, "v2.0.8", cv.Version)
	assert.Equal(t, "registry/charts/crossplane:2.0.8@sha256:aaa", cv.OCIURL)
	assert.Equal(t, "registry/images/crossplane:v2.0.8@sha256:bbb", cv.DockerRef)
	assert.Equal(t, "xp-pull", pullSecrets["v2.0.8"])
}

func TestBuildComponentVersions_Provider(t *testing.T) {
	// A provider component version has only an ociImage resource and no pull secret.
	components := []Component{
		{
			Name:    "github.com/openmcp-project/releasechannel/provider-kubernetes",
			Version: "v1.2.1",
			Resources: []Resource{
				{Name: "provider-kubernetes", Type: TypeOCIImage, Access: Access{ImageReference: "registry/provider-kubernetes:v1.2.1@sha256:ccc"}},
			},
		},
	}

	versions, pullSecrets := BuildComponentVersions(components)

	assert.Len(t, versions, 1)
	cv := versions["v1.2.1"]
	assert.Equal(t, "v1.2.1", cv.Version)
	assert.Empty(t, cv.OCIURL)
	assert.Equal(t, "registry/provider-kubernetes:v1.2.1@sha256:ccc", cv.DockerRef)
	assert.Empty(t, pullSecrets)
}

func TestToComponents_Crossplane(t *testing.T) {
	entries := []Entry{
		{ComponentName: "github.com/openmcp-project/releasechannel/crossplane", ComponentVersion: "v2.0.8", ImageRef: "registry/charts/crossplane:2.0.8@sha256:aaa", Name: "crossplane"},
		{ComponentName: "github.com/openmcp-project/releasechannel/crossplane", ComponentVersion: "v2.0.8", ImageRef: "registry/images/crossplane:v2.0.8@sha256:bbb", Name: "image-crossplane"},
	}

	comps := ToComponents(entries, "xp-secret")
	require_len(t, comps, 1)
	assert.Equal(t, "v2.0.8", comps[0].Version)
	assert.Equal(t, "xp-secret", comps[0].PullSecret)
	require_len(t, comps[0].Resources, 2)

	// "crossplane" has an image- counterpart → helmChart
	assert.Equal(t, TypeHelmChart, comps[0].Resources[0].Type)
	assert.Equal(t, "registry/charts/crossplane:2.0.8@sha256:aaa", comps[0].Resources[0].Access.ImageReference)
	// "image-crossplane" → ociImage
	assert.Equal(t, TypeOCIImage, comps[0].Resources[1].Type)
	assert.Equal(t, "registry/images/crossplane:v2.0.8@sha256:bbb", comps[0].Resources[1].Access.ImageReference)
}

func TestToComponents_Provider(t *testing.T) {
	entries := []Entry{
		{ComponentName: "github.com/openmcp-project/releasechannel/provider-kubernetes", ComponentVersion: "v1.2.1", ImageRef: "registry/provider-kubernetes:v1.2.1@sha256:ccc", Name: "provider-kubernetes"},
	}

	comps := ToComponents(entries, "")
	require_len(t, comps, 1)
	assert.Equal(t, "v1.2.1", comps[0].Version)
	assert.Empty(t, comps[0].PullSecret)
	require_len(t, comps[0].Resources, 1)
	// No image- counterpart → ociImage
	assert.Equal(t, TypeOCIImage, comps[0].Resources[0].Type)
}

func TestToComponents_ExplicitType(t *testing.T) {
	entries := []Entry{
		{ComponentName: "comp", ComponentVersion: "v1", ImageRef: "chart:1", Name: "foo", Type: "helmChart"},
	}

	comps := ToComponents(entries, "")
	require_len(t, comps[0].Resources, 1)
	assert.Equal(t, TypeHelmChart, comps[0].Resources[0].Type)
}

func TestSortVersions(t *testing.T) {
	vs := []string{"v1.20.0", "v1.9.0", "v2.0.8", "latest", "v1.10.0"}
	sortVersions(vs)
	// Valid semver ascending first, unparseable ("latest") last.
	assert.Equal(t, []string{"v1.9.0", "v1.10.0", "v1.20.0", "v2.0.8", "latest"}, vs)
}

func TestStore_SetResolveDelete(t *testing.T) {
	s := NewStore()

	_, ok := s.Resolve("crossplane", "v1.0.0")
	assert.False(t, ok)

	s.Set("crossplane", map[string]v1beta1.ComponentVersion{
		"v1.0.0": {Version: "v1.0.0", OCIURL: "chart:1.0.0"},
		"v2.0.0": {Version: "v2.0.0", OCIURL: "chart:2.0.0"},
	}, map[string]string{"v1.0.0": "secret-a"})

	cv, ok := s.Resolve("crossplane", "v1.0.0")
	assert.True(t, ok)
	assert.Equal(t, "chart:1.0.0", cv.OCIURL)

	assert.Equal(t, []string{"v1.0.0", "v2.0.0"}, s.AvailableVersions("crossplane"))
	assert.Equal(t, "secret-a", s.PullSecret("crossplane", "v1.0.0"))
	assert.Empty(t, s.PullSecret("crossplane", "v2.0.0"))
	assert.True(t, s.HasPullSecret("secret-a"))
	assert.False(t, s.HasPullSecret("nope"))
	assert.False(t, s.HasPullSecret(""))

	s.Delete("crossplane")
	_, ok = s.Resolve("crossplane", "v1.0.0")
	assert.False(t, ok)
	assert.False(t, s.HasPullSecret("secret-a"))
}

func TestStore_SetEmptyDeletes(t *testing.T) {
	s := NewStore()
	s.Set("crossplane", map[string]v1beta1.ComponentVersion{"v1.0.0": {Version: "v1.0.0"}}, nil)
	s.Set("crossplane", nil, nil)
	assert.Empty(t, s.AvailableVersions("crossplane"))
}

func TestStore_Concurrent(t *testing.T) {
	s := NewStore()
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			s.Set("crossplane", map[string]v1beta1.ComponentVersion{"v1.0.0": {Version: "v1.0.0"}}, nil)
		}()
		go func() {
			defer wg.Done()
			_, _ = s.Resolve("crossplane", "v1.0.0")
			_ = s.AvailableVersions("crossplane")
			_ = s.HasPullSecret("x")
		}()
	}
	wg.Wait()
}

func require_len(t *testing.T, slice interface{}, length int) {
	t.Helper()
	switch s := slice.(type) {
	case []Component:
		assert.Len(t, s, length)
	case []Resource:
		assert.Len(t, s, length)
	}
}
