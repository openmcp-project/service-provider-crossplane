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

package controller

import (
	"testing"

	commonapi "github.com/openmcp-project/openmcp-operator/api/common"
)

func TestDeduplicateSecretRefs(t *testing.T) {
	testCases := []struct {
		desc     string
		secrets  []commonapi.LocalObjectReference
		expected []commonapi.LocalObjectReference
	}{
		{
			desc:     "nil input - returns nil",
			secrets:  nil,
			expected: nil,
		},
		{
			desc:     "empty slice - returns nil",
			secrets:  []commonapi.LocalObjectReference{},
			expected: nil,
		},
		{
			desc: "single secret - returns as is",
			secrets: []commonapi.LocalObjectReference{
				{Name: "secret-1"},
			},
			expected: []commonapi.LocalObjectReference{
				{Name: "secret-1"},
			},
		},
		{
			desc: "different secrets - keeps all",
			secrets: []commonapi.LocalObjectReference{
				{Name: "secret-1"},
				{Name: "secret-2"},
				{Name: "secret-3"},
			},
			expected: []commonapi.LocalObjectReference{
				{Name: "secret-1"},
				{Name: "secret-2"},
				{Name: "secret-3"},
			},
		},
		{
			desc: "duplicate secrets - keeps only first occurrence",
			secrets: []commonapi.LocalObjectReference{
				{Name: "secret-1"},
				{Name: "secret-2"},
				{Name: "secret-1"},
				{Name: "secret-3"},
				{Name: "secret-2"},
			},
			expected: []commonapi.LocalObjectReference{
				{Name: "secret-1"},
				{Name: "secret-2"},
				{Name: "secret-3"},
			},
		},
		{
			desc: "empty secret names - filters out",
			secrets: []commonapi.LocalObjectReference{
				{Name: "secret-1"},
				{Name: ""},
				{Name: "secret-2"},
				{Name: ""},
			},
			expected: []commonapi.LocalObjectReference{
				{Name: "secret-1"},
				{Name: "secret-2"},
			},
		},
		{
			desc: "single empty name",
			secrets: []commonapi.LocalObjectReference{
				{Name: ""},
			},
			expected: nil,
		},
		{
			desc: "all empty secret names - returns nil",
			secrets: []commonapi.LocalObjectReference{
				{Name: ""},
				{Name: ""},
				{Name: ""},
			},
			expected: nil,
		},
		{
			desc: "mixed duplicates and empty names",
			secrets: []commonapi.LocalObjectReference{
				{Name: "secret-1"},
				{Name: ""},
				{Name: "secret-1"},
				{Name: "secret-2"},
				{Name: ""},
				{Name: "secret-2"},
				{Name: "secret-3"},
			},
			expected: []commonapi.LocalObjectReference{
				{Name: "secret-1"},
				{Name: "secret-2"},
				{Name: "secret-3"},
			},
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			result := deduplicateSecretRefs(tC.secrets)

			// Check length
			if len(result) != len(tC.expected) {
				t.Fatalf("expected %d secrets, got %d", len(tC.expected), len(result))
			}

			// Check each element
			for i, expectedSecret := range tC.expected {
				if result[i] != expectedSecret {
					t.Errorf("at index %d: expected secret %v, got %v", i, expectedSecret, result[i])
				}
			}
		})
	}
}
