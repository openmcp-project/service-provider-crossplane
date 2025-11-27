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
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/openmcp-project/control-plane-operator/pkg/juggler"
	"github.com/openmcp-project/control-plane-operator/pkg/utils/rcontext"
	commonapi "github.com/openmcp-project/openmcp-operator/api/common"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openmcp-project/service-provider-crossplane/api/v1alpha1"
	"github.com/openmcp-project/service-provider-crossplane/pkg/component"
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

func Test_buildComponents(t *testing.T) {
	type args struct {
		ctx             context.Context
		client          client.Client
		xp              *v1alpha1.Crossplane
		pc              *v1alpha1.ProviderConfig
		enabled         bool
		setPodNamespace bool
	}
	tests := []struct {
		name    string
		args    args
		want    []juggler.Component
		wantErr error
	}{
		{
			name: "Crossplane and crossplane-provider components are built and enabled",
			args: args{
				ctx:             rcontext.WithTenantNamespace(context.Background(), "tenant-namespace"),
				client:          nil,
				setPodNamespace: true,
				xp: &v1alpha1.Crossplane{
					Spec: v1alpha1.CrossplaneSpec{
						Version:   "v1.0.0",
						Providers: []*v1alpha1.CrossplaneProviderConfig{{Name: "provider-1", Version: "v0.1.0"}},
					},
				},
				pc: &v1alpha1.ProviderConfig{
					Spec: v1alpha1.ProviderConfigSpec{
						CrossplaneVersions: []v1alpha1.CrossplaneVersion{
							{
								Version: "v1.0.0",
								Chart:   v1alpha1.ChartSpec{URL: "https://charts.example.com/1"},
								Image:   v1alpha1.ImageSpec{URL: "https://images.example.com/1"},
							},
							{
								Version: "v2.0.0",
								Chart:   v1alpha1.ChartSpec{URL: "https://charts.example.com/2"},
								Image:   v1alpha1.ImageSpec{URL: "https://images.example.com/2"},
							},
						},
						Providers: v1alpha1.CrossplaneProviders{
							AvailableProviders: []v1alpha1.AvailableCrossplaneProvider{
								{Name: "provider-1", Versions: []string{"v0.1.0"}, Package: "crossplane/provider-aws:v0.1.0"},
								{Name: "provider-2", Versions: []string{"v0.1.0"}, Package: "crossplane/provider-other:v0.1.0"},
							},
						},
					},
				},
				enabled: true,
			},
			want: []juggler.Component{
				&component.Crossplane{
					Enabled: true,
					Config: &v1alpha1.CrossplaneSpec{
						Version:   "v1.0.0",
						Providers: []*v1alpha1.CrossplaneProviderConfig{{Name: "provider-1", Version: "v0.1.0"}},
					},
				},
				&component.CrossplaneProvider{
					Enabled: true,
					Config:  &v1alpha1.CrossplaneProviderConfig{Name: "provider-1", Version: "v0.1.0"},
				},
			},
			wantErr: nil,
		},
		{
			name: "Crossplane, crossplane-provider, platform-secret and secret components are built and enabled",
			args: args{
				ctx:             rcontext.WithTenantNamespace(context.Background(), "tenant-namespace"),
				client:          nil,
				setPodNamespace: true,
				xp: &v1alpha1.Crossplane{
					Spec: v1alpha1.CrossplaneSpec{
						Version:   "v2.0.0",
						Providers: []*v1alpha1.CrossplaneProviderConfig{{Name: "provider-1", Version: "v0.1.0"}},
					},
				},
				pc: &v1alpha1.ProviderConfig{
					Spec: v1alpha1.ProviderConfigSpec{
						CrossplaneVersions: []v1alpha1.CrossplaneVersion{
							{
								Version: "v1.0.0",
								Chart:   v1alpha1.ChartSpec{URL: "https://charts.example.com/1", SecretRef: commonapi.LocalObjectReference{Name: "chart-secret"}},
								Image:   v1alpha1.ImageSpec{URL: "https://images.example.com/1", SecretRef: commonapi.LocalObjectReference{Name: "image-secret"}},
							},
							{
								Version: "v2.0.0",
								Chart:   v1alpha1.ChartSpec{URL: "https://charts.example.com/2", SecretRef: commonapi.LocalObjectReference{Name: "other-chart-secret"}},
								Image:   v1alpha1.ImageSpec{URL: "https://images.example.com/2", SecretRef: commonapi.LocalObjectReference{Name: "other-image-secret"}},
							},
						},
						Providers: v1alpha1.CrossplaneProviders{
							AvailableProviders: []v1alpha1.AvailableCrossplaneProvider{
								{Name: "provider-1", Versions: []string{"v0.1.0"}, Package: "crossplane/provider-aws:v0.1.0"},
								{Name: "provider-2", Versions: []string{"v0.1.0"}, Package: "crossplane/provider-other:v0.1.0"},
							},
							ImagePullSecrets: []commonapi.LocalObjectReference{{Name: "provider-image-secret"}, {Name: "other-provider-image-secret"}},
						},
					},
				},
				enabled: true,
			},
			want: []juggler.Component{
				// Components expected to be built containing ALL (platform)secrets from providerConfig,
				// regardless of whether they are used by Crossplane or its providers
				&component.Crossplane{
					Enabled:              true,
					Config:               &v1alpha1.CrossplaneSpec{Version: "v2.0.0", Providers: []*v1alpha1.CrossplaneProviderConfig{{Name: "provider-1", Version: "v0.1.0"}}},
					ChartPullSecretName:  "other-chart-secret",
					ImagePullSecretNames: []string{"other-image-secret"},
				},
				&component.CrossplaneProvider{
					Enabled:     true,
					Config:      &v1alpha1.CrossplaneProviderConfig{Name: "provider-1", Version: "v0.1.0"},
					PullSecrets: []corev1.LocalObjectReference{{Name: "provider-image-secret"}, {Name: "other-provider-image-secret"}},
				},
				&component.PlatformSecret{
					SourceClient: nil,
					Source:       client.ObjectKey{Name: "other-chart-secret", Namespace: "pod-namespace"},
					Target:       client.ObjectKey{Name: "other-chart-secret", Namespace: "tenant-namespace"},
					Enabled:      true,
				},
				&component.Secret{
					SourceClient: nil,
					Source:       client.ObjectKey{Name: "other-image-secret", Namespace: "pod-namespace"},
					Target:       client.ObjectKey{Name: "other-image-secret", Namespace: component.CrossplaneNamespace},
					Enabled:      true,
				},
				&component.Secret{
					SourceClient: nil,
					Source:       client.ObjectKey{Name: "provider-image-secret", Namespace: "pod-namespace"},
					Target:       client.ObjectKey{Name: "provider-image-secret", Namespace: component.CrossplaneNamespace},
					Enabled:      true,
				},
				&component.Secret{
					SourceClient: nil,
					Source:       client.ObjectKey{Name: "other-provider-image-secret", Namespace: "pod-namespace"},
					Target:       client.ObjectKey{Name: "other-provider-image-secret", Namespace: component.CrossplaneNamespace},
					Enabled:      true,
				},
			},
			wantErr: nil,
		},
		{
			name: "Crossplane components are built and enabled, duplicate secret components removed",
			args: args{
				ctx:             rcontext.WithTenantNamespace(context.Background(), "tenant-namespace"),
				client:          nil,
				setPodNamespace: true,
				xp: &v1alpha1.Crossplane{
					Spec: v1alpha1.CrossplaneSpec{
						Version:   "v1.0.0",
						Providers: []*v1alpha1.CrossplaneProviderConfig{{Name: "provider-1", Version: "v0.1.0"}},
					},
				},
				pc: &v1alpha1.ProviderConfig{
					Spec: v1alpha1.ProviderConfigSpec{
						CrossplaneVersions: []v1alpha1.CrossplaneVersion{
							{
								Version: "v1.0.0",
								Chart:   v1alpha1.ChartSpec{URL: "https://charts.example.com/1", SecretRef: commonapi.LocalObjectReference{Name: "chart-secret"}},
								Image:   v1alpha1.ImageSpec{URL: "https://images.example.com/1", SecretRef: commonapi.LocalObjectReference{Name: "image-secret"}},
							},
							{
								Version: "v2.0.0",
								Chart:   v1alpha1.ChartSpec{URL: "https://charts.example.com/2", SecretRef: commonapi.LocalObjectReference{Name: "chart-secret"}},
								Image:   v1alpha1.ImageSpec{URL: "https://images.example.com/2", SecretRef: commonapi.LocalObjectReference{Name: "image-secret"}},
							},
						},
						Providers: v1alpha1.CrossplaneProviders{
							AvailableProviders: []v1alpha1.AvailableCrossplaneProvider{
								{Name: "provider-1", Versions: []string{"v0.1.0"}, Package: "crossplane/provider-aws:v0.1.0"},
								{Name: "provider-2", Versions: []string{"v0.1.0"}, Package: "crossplane/provider-other:v0.1.0"},
							},
							ImagePullSecrets: []commonapi.LocalObjectReference{{Name: "image-secret"}},
						},
					},
				},
				enabled: true,
			},
			want: []juggler.Component{
				// Components expected to be built containing ALL (platform)secrets from providerConfig,
				// regardless of whether they are used by Crossplane or its providers. Duplicates removed.
				&component.Crossplane{
					Enabled: true,
					Config: &v1alpha1.CrossplaneSpec{
						Version:   "v1.0.0",
						Providers: []*v1alpha1.CrossplaneProviderConfig{{Name: "provider-1", Version: "v0.1.0"}},
					},
					ChartPullSecretName:  "chart-secret",
					ImagePullSecretNames: []string{"image-secret"},
				},
				&component.CrossplaneProvider{
					Enabled:     true,
					Config:      &v1alpha1.CrossplaneProviderConfig{Name: "provider-1", Version: "v0.1.0"},
					PullSecrets: []corev1.LocalObjectReference{{Name: "image-secret"}},
				},
				&component.PlatformSecret{
					SourceClient: nil,
					Source:       client.ObjectKey{Name: "chart-secret", Namespace: "pod-namespace"},
					Target:       client.ObjectKey{Name: "chart-secret", Namespace: "tenant-namespace"},
					Enabled:      true,
				},
				&component.Secret{
					SourceClient: nil,
					Source:       client.ObjectKey{Name: "image-secret", Namespace: "pod-namespace"},
					Target:       client.ObjectKey{Name: "image-secret", Namespace: component.CrossplaneNamespace},
					Enabled:      true,
				},
			},
			wantErr: nil,
		},
		{
			name: "Components are built and disabled (for deletion)",
			args: args{
				ctx:             rcontext.WithTenantNamespace(context.Background(), "tenant-namespace"),
				client:          nil,
				setPodNamespace: true,
				xp: &v1alpha1.Crossplane{
					Spec: v1alpha1.CrossplaneSpec{
						Version:   "v1.0.0",
						Providers: []*v1alpha1.CrossplaneProviderConfig{{Name: "provider-1", Version: "v0.1.0"}},
					},
				},
				pc: &v1alpha1.ProviderConfig{
					Spec: v1alpha1.ProviderConfigSpec{
						CrossplaneVersions: []v1alpha1.CrossplaneVersion{
							{
								Version: "v1.0.0",
								Chart:   v1alpha1.ChartSpec{URL: "https://charts.example.com/foo", SecretRef: commonapi.LocalObjectReference{Name: "chart-secret"}},
								Image:   v1alpha1.ImageSpec{URL: "https://images.example.com/foo", SecretRef: commonapi.LocalObjectReference{Name: "image-secret"}},
							},
							{
								Version: "v2.0.0",
								Chart:   v1alpha1.ChartSpec{URL: "https://charts.example.com/2", SecretRef: commonapi.LocalObjectReference{Name: "other-chart-secret"}},
								Image:   v1alpha1.ImageSpec{URL: "https://images.example.com/2", SecretRef: commonapi.LocalObjectReference{Name: "other-image-secret"}},
							},
						},
						Providers: v1alpha1.CrossplaneProviders{
							AvailableProviders: []v1alpha1.AvailableCrossplaneProvider{
								{Name: "provider-1", Versions: []string{"v0.1.0"}, Package: "crossplane/provider-aws:v0.1.0"},
								{Name: "provider-2", Versions: []string{"v0.1.0"}, Package: "crossplane/provider-other:v0.1.0"},
							},
							ImagePullSecrets: []commonapi.LocalObjectReference{{Name: "provider-image-secret"}, {Name: "other-provider-image-secret"}},
						},
					},
				},
				enabled: false,
			},
			want: []juggler.Component{
				// Components expected to be built containing ALL (platform)secrets from providerConfig,
				// regardless of whether they are used by Crossplane or its providers
				&component.Crossplane{
					Enabled: false,
					Config: &v1alpha1.CrossplaneSpec{
						Version:   "v1.0.0",
						Providers: []*v1alpha1.CrossplaneProviderConfig{{Name: "provider-1", Version: "v0.1.0"}},
					},
					ChartPullSecretName:  "chart-secret",
					ImagePullSecretNames: []string{"image-secret"},
				},
				&component.CrossplaneProvider{
					Enabled:     false,
					Config:      &v1alpha1.CrossplaneProviderConfig{Name: "provider-1", Version: "v0.1.0"},
					PullSecrets: []corev1.LocalObjectReference{{Name: "provider-image-secret"}, {Name: "other-provider-image-secret"}},
				},
				&component.PlatformSecret{
					SourceClient: nil,
					Source:       client.ObjectKey{Name: "chart-secret", Namespace: "pod-namespace"},
					Target:       client.ObjectKey{Name: "chart-secret", Namespace: "tenant-namespace"},
					Enabled:      false,
				},
				&component.Secret{
					SourceClient: nil,
					Source:       client.ObjectKey{Name: "image-secret", Namespace: "pod-namespace"},
					Target:       client.ObjectKey{Name: "image-secret", Namespace: component.CrossplaneNamespace},
					Enabled:      false,
				},
				&component.Secret{
					SourceClient: nil,
					Source:       client.ObjectKey{Name: "provider-image-secret", Namespace: "pod-namespace"},
					Target:       client.ObjectKey{Name: "provider-image-secret", Namespace: component.CrossplaneNamespace},
					Enabled:      false,
				},
				&component.Secret{
					SourceClient: nil,
					Source:       client.ObjectKey{Name: "other-provider-image-secret", Namespace: "pod-namespace"},
					Target:       client.ObjectKey{Name: "other-provider-image-secret", Namespace: component.CrossplaneNamespace},
					Enabled:      false,
				},
			},
			wantErr: nil,
		},
		{
			name: "Error when POD_NAMESPACE evn var not found",
			args: args{
				ctx:             rcontext.WithTenantNamespace(context.Background(), "tenant-namespace"),
				client:          nil,
				setPodNamespace: false,
				xp: &v1alpha1.Crossplane{
					Spec: v1alpha1.CrossplaneSpec{
						Version:   "v1.0.0",
						Providers: []*v1alpha1.CrossplaneProviderConfig{{Name: "provider-1", Version: "v0.1.0"}},
					},
				},
				pc: &v1alpha1.ProviderConfig{
					Spec: v1alpha1.ProviderConfigSpec{
						CrossplaneVersions: []v1alpha1.CrossplaneVersion{
							{
								Version: "v1.0.0",
								Chart:   v1alpha1.ChartSpec{URL: "https://charts.example.com/foo", SecretRef: commonapi.LocalObjectReference{Name: "chart-secret"}},
								Image:   v1alpha1.ImageSpec{URL: "https://images.example.com/foo", SecretRef: commonapi.LocalObjectReference{Name: "image-secret"}},
							},
							{
								Version: "v2.0.0",
								Chart:   v1alpha1.ChartSpec{URL: "https://charts.example.com/2", SecretRef: commonapi.LocalObjectReference{Name: "other-chart-secret"}},
								Image:   v1alpha1.ImageSpec{URL: "https://images.example.com/2", SecretRef: commonapi.LocalObjectReference{Name: "other-image-secret"}},
							},
						},
						Providers: v1alpha1.CrossplaneProviders{
							AvailableProviders: []v1alpha1.AvailableCrossplaneProvider{
								{Name: "provider-1", Versions: []string{"v0.1.0"}, Package: "crossplane/provider-aws:v0.1.0"},
								{Name: "provider-2", Versions: []string{"v0.1.0"}, Package: "crossplane/provider-other:v0.1.0"},
							},
							ImagePullSecrets: []commonapi.LocalObjectReference{{Name: "provider-image-secret"}, {Name: "other-provider-image-secret"}},
						},
					},
				},
				enabled: true,
			},
			want:    nil,
			wantErr: errors.New("environment variable POD_NAMESPACE not set - cannot determine source namespace for secrets"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.args.setPodNamespace {
				t.Setenv("POD_NAMESPACE", "pod-namespace")
			}
			got, err := buildComponents(tt.args.ctx, tt.args.client, tt.args.xp, tt.args.pc, tt.args.enabled)
			if err != nil && tt.wantErr == nil && err.Error() != tt.wantErr.Error() {
				t.Errorf("buildComponents() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("buildComponents() length mismatch: got %d, want %d", len(got), len(tt.want))
				return
			}
			// Check if each element in got has a counterpart in want
			for i, gotComponent := range got {
				found := false
				for _, wantComponent := range tt.want {
					if reflect.DeepEqual(gotComponent, wantComponent) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("buildComponents() element %d not found in expected: %v", i, gotComponent)
				}
			}
		})
	}
}
