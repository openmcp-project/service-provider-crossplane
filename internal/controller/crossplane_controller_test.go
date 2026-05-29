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
	"fmt"
	"reflect"
	"strings"
	"testing"

	crossplanev1beta1 "github.com/crossplane/crossplane/apis/v2/pkg/v1beta1"
	"github.com/openmcp-project/control-plane-operator/pkg/juggler"
	"github.com/openmcp-project/control-plane-operator/pkg/utils/rcontext"
	"github.com/openmcp-project/controller-utils/pkg/clusters"
	errutils "github.com/openmcp-project/controller-utils/pkg/errors"
	commonapi "github.com/openmcp-project/openmcp-operator/api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/openmcp-project/service-provider-crossplane/api/v1alpha1"
	"github.com/openmcp-project/service-provider-crossplane/internal/scheme"
	"github.com/openmcp-project/service-provider-crossplane/pkg/component"
)

const TestProviderName = "crossplane-test"

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
								Image:   &v1alpha1.ImageSpec{URL: "https://images.example.com/1"},
							},
							{
								Version: "v2.0.0",
								Chart:   v1alpha1.ChartSpec{URL: "https://charts.example.com/2"},
								Image:   &v1alpha1.ImageSpec{URL: "https://images.example.com/2"},
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
				&component.DeploymentRuntimeConfig{
					Enabled: true,
					Name:    "default",
					Config:  &crossplanev1beta1.DeploymentRuntimeConfigSpec{},
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
								Image:   &v1alpha1.ImageSpec{URL: "https://images.example.com/1", SecretRef: commonapi.LocalObjectReference{Name: "image-secret"}},
							},
							{
								Version: "v2.0.0",
								Chart:   v1alpha1.ChartSpec{URL: "https://charts.example.com/2", SecretRef: commonapi.LocalObjectReference{Name: "other-chart-secret"}},
								Image:   &v1alpha1.ImageSpec{URL: "https://images.example.com/2", SecretRef: commonapi.LocalObjectReference{Name: "other-image-secret"}},
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
					ChartPullSecretName:  fmt.Sprintf("%s%s", secretNamePrefix, "other-chart-secret"),
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
					Target:       client.ObjectKey{Name: fmt.Sprintf("%s%s", secretNamePrefix, "other-chart-secret"), Namespace: "tenant-namespace"},
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
				&component.DeploymentRuntimeConfig{
					Enabled: true,
					Name:    "default",
					Config:  &crossplanev1beta1.DeploymentRuntimeConfigSpec{},
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
								Image:   &v1alpha1.ImageSpec{URL: "https://images.example.com/1", SecretRef: commonapi.LocalObjectReference{Name: "image-secret"}},
							},
							{
								Version: "v2.0.0",
								Chart:   v1alpha1.ChartSpec{URL: "https://charts.example.com/2", SecretRef: commonapi.LocalObjectReference{Name: "chart-secret"}},
								Image:   &v1alpha1.ImageSpec{URL: "https://images.example.com/2", SecretRef: commonapi.LocalObjectReference{Name: "image-secret"}},
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
					ChartPullSecretName:  fmt.Sprintf("%s%s", secretNamePrefix, "chart-secret"),
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
					Target:       client.ObjectKey{Name: fmt.Sprintf("%s%s", secretNamePrefix, "chart-secret"), Namespace: "tenant-namespace"},
					Enabled:      true,
				},
				&component.Secret{
					SourceClient: nil,
					Source:       client.ObjectKey{Name: "image-secret", Namespace: "pod-namespace"},
					Target:       client.ObjectKey{Name: "image-secret", Namespace: component.CrossplaneNamespace},
					Enabled:      true,
				},
				&component.DeploymentRuntimeConfig{
					Enabled: true,
					Name:    "default",
					Config:  &crossplanev1beta1.DeploymentRuntimeConfigSpec{},
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
								Image:   &v1alpha1.ImageSpec{URL: "https://images.example.com/foo", SecretRef: commonapi.LocalObjectReference{Name: "image-secret"}},
							},
							{
								Version: "v2.0.0",
								Chart:   v1alpha1.ChartSpec{URL: "https://charts.example.com/2", SecretRef: commonapi.LocalObjectReference{Name: "other-chart-secret"}},
								Image:   &v1alpha1.ImageSpec{URL: "https://images.example.com/2", SecretRef: commonapi.LocalObjectReference{Name: "other-image-secret"}},
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
					ChartPullSecretName:  fmt.Sprintf("%s%s", secretNamePrefix, "chart-secret"),
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
					Target:       client.ObjectKey{Name: fmt.Sprintf("%s%s", secretNamePrefix, "chart-secret"), Namespace: "tenant-namespace"},
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
				&component.DeploymentRuntimeConfig{
					Enabled: false,
					Name:    "default",
					Config:  &crossplanev1beta1.DeploymentRuntimeConfigSpec{},
				},
			},
			wantErr: nil,
		},
		{
			name: "Crossplane with custom CA bundle components are built and enabled",
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
								Image:   &v1alpha1.ImageSpec{URL: "https://images.example.com/1"},
							},
						},
						Providers: v1alpha1.CrossplaneProviders{
							AvailableProviders: []v1alpha1.AvailableCrossplaneProvider{
								{Name: "provider-1", Versions: []string{"v0.1.0"}, Package: "crossplane/provider-aws:v0.1.0"},
							},
						},
						CABundleRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "my-ca-bundle",
							},
							Key: "ca-bundle.crt",
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
					CABundleRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "my-ca-bundle",
						},
						Key: "ca-bundle.crt",
					},
				},
				&component.CrossplaneProvider{
					Enabled: true,
					Config:  &v1alpha1.CrossplaneProviderConfig{Name: "provider-1", Version: "v0.1.0"},
				},
				&component.DeploymentRuntimeConfig{
					Enabled: true,
					Name:    "default",
					Config: &crossplanev1beta1.DeploymentRuntimeConfigSpec{
						DeploymentTemplate: &crossplanev1beta1.DeploymentTemplate{
							Spec: &appsv1.DeploymentSpec{
								Selector: &metav1.LabelSelector{},
								Template: corev1.PodTemplateSpec{
									Spec: corev1.PodSpec{
										Containers: []corev1.Container{
											{
												Name: "package-runtime",
												VolumeMounts: []corev1.VolumeMount{
													{
														MountPath: "/etc/custom-ca",
														Name:      "custom-ca-bundle",
														ReadOnly:  true,
													},
												},
												Env: []corev1.EnvVar{
													{
														Name:  "SSL_CERT_FILE",
														Value: "/etc/custom-ca/ca-bundle.crt",
													},
													{
														Name:  "SSL_CERT_DIR",
														Value: "/etc/custom-ca",
													},
												},
											},
										},
										Volumes: []corev1.Volume{
											{
												Name: "custom-ca-bundle",
												VolumeSource: corev1.VolumeSource{
													ConfigMap: &corev1.ConfigMapVolumeSource{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: "custom-ca-bundle",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				&component.ConfigMap{
					Enabled:      true,
					SourceClient: nil,
					Source: client.ObjectKey{
						Name:      "my-ca-bundle",
						Namespace: "pod-namespace",
					},
					Target: client.ObjectKey{
						Name:      "custom-ca-bundle",
						Namespace: "crossplane-system",
					},
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
								Image:   &v1alpha1.ImageSpec{URL: "https://images.example.com/foo", SecretRef: commonapi.LocalObjectReference{Name: "image-secret"}},
							},
							{
								Version: "v2.0.0",
								Chart:   v1alpha1.ChartSpec{URL: "https://charts.example.com/2", SecretRef: commonapi.LocalObjectReference{Name: "other-chart-secret"}},
								Image:   &v1alpha1.ImageSpec{URL: "https://images.example.com/2", SecretRef: commonapi.LocalObjectReference{Name: "other-image-secret"}},
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

func Test_extractHelmChartPullSecretForVersion(t *testing.T) {
	versions := []v1alpha1.CrossplaneVersion{
		{Version: "v1.0.0", Chart: v1alpha1.ChartSpec{SecretRef: commonapi.LocalObjectReference{Name: "secret-1"}}},
		{Version: "v2.0.0", Chart: v1alpha1.ChartSpec{SecretRef: commonapi.LocalObjectReference{Name: "secret-2"}}},
	}

	_, err := extractHelmChartPullSecretForVersion("v3.0.0", versions)
	assert.Error(t, err)
	assert.Nil(t, errutils.IgnoreInvalidUserInput(err), "unsupported version error should be treated as invalid user input")
}

func Test_GetResolverFunc(t *testing.T) {
	pc := &v1alpha1.ProviderConfig{
		Spec: v1alpha1.ProviderConfigSpec{
			CrossplaneVersions: []v1alpha1.CrossplaneVersion{
				{Version: "v1.0.0", Chart: v1alpha1.ChartSpec{URL: "oci://charts/crossplane:v1.0.0"}},
			},
			Providers: v1alpha1.CrossplaneProviders{
				AvailableProviders: []v1alpha1.AvailableCrossplaneProvider{
					{Name: "provider-aws", Versions: []string{"v0.1.0"}, Package: "crossplane/provider-aws"},
				},
			},
		},
	}
	r := &CrossplaneReconciler{}
	resolve := r.GetResolverFunc(pc)

	tests := []struct {
		name          string
		componentName string
		version       string
		wantErr       bool
	}{
		{"valid crossplane version", component.CrossplaneRelease, "v1.0.0", false},
		{"unsupported crossplane version", component.CrossplaneRelease, "v9.9.9", true},
		{"valid provider version", "provider-aws", "v0.1.0", false},
		{"unsupported provider version", "provider-aws", "v9.9.9", true},
		{"unknown provider", "provider-unknown", "v0.1.0", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolve(tt.componentName, tt.version)
			if !tt.wantErr {
				assert.NoError(t, err)
				return
			}
			assert.Error(t, err)
			assert.Nil(t, errutils.IgnoreInvalidUserInput(err), "version resolver error should be treated as invalid user input")
		})
	}
}

func Test_isSecretReferencedInProviderConfig(t *testing.T) {
	tests := []struct {
		name       string
		pc         *v1alpha1.ProviderConfig
		secretName string
		want       bool
	}{
		{
			name: "matches chart pull secret",
			pc: &v1alpha1.ProviderConfig{
				Spec: v1alpha1.ProviderConfigSpec{
					CrossplaneVersions: []v1alpha1.CrossplaneVersion{
						{
							Version: "v1.0.0",
							Chart:   v1alpha1.ChartSpec{URL: "https://charts.example.com/1", SecretRef: commonapi.LocalObjectReference{Name: "chart-secret"}},
							Image:   &v1alpha1.ImageSpec{URL: "https://images.example.com/1"},
						},
					},
					Providers: v1alpha1.CrossplaneProviders{},
				},
			},
			secretName: "chart-secret",
			want:       true,
		},
		{
			name: "matches image pull secret",
			pc: &v1alpha1.ProviderConfig{
				Spec: v1alpha1.ProviderConfigSpec{
					CrossplaneVersions: []v1alpha1.CrossplaneVersion{
						{
							Version: "v1.0.0",
							Chart:   v1alpha1.ChartSpec{URL: "https://charts.example.com/1"},
							Image:   &v1alpha1.ImageSpec{URL: "https://images.example.com/1", SecretRef: commonapi.LocalObjectReference{Name: "image-secret"}},
						},
					},
					Providers: v1alpha1.CrossplaneProviders{},
				},
			},
			secretName: "image-secret",
			want:       true,
		},
		{
			name: "matches provider image pull secret",
			pc: &v1alpha1.ProviderConfig{
				Spec: v1alpha1.ProviderConfigSpec{
					CrossplaneVersions: []v1alpha1.CrossplaneVersion{
						{
							Version: "v1.0.0",
							Chart:   v1alpha1.ChartSpec{URL: "https://charts.example.com/1"},
							Image:   &v1alpha1.ImageSpec{URL: "https://images.example.com/1"},
						},
					},
					Providers: v1alpha1.CrossplaneProviders{
						ImagePullSecrets: []commonapi.LocalObjectReference{
							{Name: "provider-pull-secret"},
						},
					},
				},
			},
			secretName: "provider-pull-secret",
			want:       true,
		},
		{
			name: "matches secret in second version entry",
			pc: &v1alpha1.ProviderConfig{
				Spec: v1alpha1.ProviderConfigSpec{
					CrossplaneVersions: []v1alpha1.CrossplaneVersion{
						{
							Version: "v1.0.0",
							Chart:   v1alpha1.ChartSpec{URL: "https://charts.example.com/1"},
							Image:   &v1alpha1.ImageSpec{URL: "https://images.example.com/1"},
						},
						{
							Version: "v2.0.0",
							Chart:   v1alpha1.ChartSpec{URL: "https://charts.example.com/2", SecretRef: commonapi.LocalObjectReference{Name: "v2-chart-secret"}},
							Image:   &v1alpha1.ImageSpec{URL: "https://images.example.com/2"},
						},
					},
					Providers: v1alpha1.CrossplaneProviders{},
				},
			},
			secretName: "v2-chart-secret",
			want:       true,
		},
		{
			name: "does not match unrelated secret",
			pc: &v1alpha1.ProviderConfig{
				Spec: v1alpha1.ProviderConfigSpec{
					CrossplaneVersions: []v1alpha1.CrossplaneVersion{
						{
							Version: "v1.0.0",
							Chart:   v1alpha1.ChartSpec{URL: "https://charts.example.com/1", SecretRef: commonapi.LocalObjectReference{Name: "chart-secret"}},
							Image:   &v1alpha1.ImageSpec{URL: "https://images.example.com/1", SecretRef: commonapi.LocalObjectReference{Name: "image-secret"}},
						},
					},
					Providers: v1alpha1.CrossplaneProviders{
						ImagePullSecrets: []commonapi.LocalObjectReference{
							{Name: "provider-pull-secret"},
						},
					},
				},
			},
			secretName: "unrelated-secret",
			want:       false,
		},
		{
			name: "empty provider config - no secrets referenced",
			pc: &v1alpha1.ProviderConfig{
				Spec: v1alpha1.ProviderConfigSpec{
					CrossplaneVersions: []v1alpha1.CrossplaneVersion{},
					Providers:          v1alpha1.CrossplaneProviders{},
				},
			},
			secretName: "any-secret",
			want:       false,
		},
		{
			name: "empty secret refs - no match",
			pc: &v1alpha1.ProviderConfig{
				Spec: v1alpha1.ProviderConfigSpec{
					CrossplaneVersions: []v1alpha1.CrossplaneVersion{
						{
							Version: "v1.0.0",
							Chart:   v1alpha1.ChartSpec{URL: "https://charts.example.com/1"},
							Image:   &v1alpha1.ImageSpec{URL: "https://images.example.com/1"},
						},
					},
					Providers: v1alpha1.CrossplaneProviders{},
				},
			},
			secretName: "some-secret",
			want:       false,
		},
		{
			name: "nil image - only chart secret checked",
			pc: &v1alpha1.ProviderConfig{
				Spec: v1alpha1.ProviderConfigSpec{
					CrossplaneVersions: []v1alpha1.CrossplaneVersion{
						{
							Version: "v1.0.0",
							Chart:   v1alpha1.ChartSpec{URL: "https://charts.example.com/1", SecretRef: commonapi.LocalObjectReference{Name: "chart-secret"}},
						},
					},
					Providers: v1alpha1.CrossplaneProviders{},
				},
			},
			secretName: "chart-secret",
			want:       true,
		},
		{
			name: "nil image - image secret does not match",
			pc: &v1alpha1.ProviderConfig{
				Spec: v1alpha1.ProviderConfigSpec{
					CrossplaneVersions: []v1alpha1.CrossplaneVersion{
						{
							Version: "v1.0.0",
							Chart:   v1alpha1.ChartSpec{URL: "https://charts.example.com/1"},
						},
					},
					Providers: v1alpha1.CrossplaneProviders{},
				},
			},
			secretName: "image-secret",
			want:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSecretReferencedInProviderConfig(tt.pc, tt.secretName)
			if got != tt.want {
				t.Errorf("isSecretReferencedInProviderConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_mapSecretToRequests(t *testing.T) {
	tests := []struct {
		name                string
		secret              *corev1.Secret
		providerConfig      *v1alpha1.ProviderConfig
		crossplaneInstances []client.Object
		wantRequests        []ctrl.Request
	}{
		{
			name: "referenced secret triggers reconciliation for all Crossplane instances",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "chart-secret",
					Namespace: "sp-namespace",
				},
			},
			providerConfig: &v1alpha1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: TestProviderName,
				},
				Spec: v1alpha1.ProviderConfigSpec{
					CrossplaneVersions: []v1alpha1.CrossplaneVersion{
						{
							Version: "v1.0.0",
							Chart:   v1alpha1.ChartSpec{URL: "https://charts.example.com/1", SecretRef: commonapi.LocalObjectReference{Name: "chart-secret"}},
							Image:   &v1alpha1.ImageSpec{URL: "https://images.example.com/1"},
						},
					},
					Providers: v1alpha1.CrossplaneProviders{},
				},
			},
			crossplaneInstances: []client.Object{
				&v1alpha1.Crossplane{
					ObjectMeta: metav1.ObjectMeta{Name: "xp-1", Namespace: "ns-1"},
				},
				&v1alpha1.Crossplane{
					ObjectMeta: metav1.ObjectMeta{Name: "xp-2", Namespace: "ns-2"},
				},
			},
			wantRequests: []ctrl.Request{
				{NamespacedName: client.ObjectKey{Name: "xp-1", Namespace: "ns-1"}},
				{NamespacedName: client.ObjectKey{Name: "xp-2", Namespace: "ns-2"}},
			},
		},
		{
			name: "unreferenced secret returns no requests",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unrelated-secret",
					Namespace: "sp-namespace",
				},
			},
			providerConfig: &v1alpha1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: TestProviderName,
				},
				Spec: v1alpha1.ProviderConfigSpec{
					CrossplaneVersions: []v1alpha1.CrossplaneVersion{
						{
							Version: "v1.0.0",
							Chart:   v1alpha1.ChartSpec{URL: "https://charts.example.com/1", SecretRef: commonapi.LocalObjectReference{Name: "chart-secret"}},
							Image:   &v1alpha1.ImageSpec{URL: "https://images.example.com/1"},
						},
					},
					Providers: v1alpha1.CrossplaneProviders{},
				},
			},
			crossplaneInstances: []client.Object{
				&v1alpha1.Crossplane{
					ObjectMeta: metav1.ObjectMeta{Name: "xp-1", Namespace: "ns-1"},
				},
			},
			wantRequests: nil,
		},
		{
			name: "referenced secret with no Crossplane instances returns empty",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "chart-secret",
					Namespace: "sp-namespace",
				},
			},
			providerConfig: &v1alpha1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: TestProviderName,
				},
				Spec: v1alpha1.ProviderConfigSpec{
					CrossplaneVersions: []v1alpha1.CrossplaneVersion{
						{
							Version: "v1.0.0",
							Chart:   v1alpha1.ChartSpec{URL: "https://charts.example.com/1", SecretRef: commonapi.LocalObjectReference{Name: "chart-secret"}},
							Image:   &v1alpha1.ImageSpec{URL: "https://images.example.com/1"},
						},
					},
					Providers: v1alpha1.CrossplaneProviders{},
				},
			},
			crossplaneInstances: nil,
			wantRequests:        []ctrl.Request{},
		},
		{
			name: "provider image pull secret triggers reconciliation",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "provider-pull-secret",
					Namespace: "sp-namespace",
				},
			},
			providerConfig: &v1alpha1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: TestProviderName,
				},
				Spec: v1alpha1.ProviderConfigSpec{
					CrossplaneVersions: []v1alpha1.CrossplaneVersion{
						{
							Version: "v1.0.0",
							Chart:   v1alpha1.ChartSpec{URL: "https://charts.example.com/1"},
							Image:   &v1alpha1.ImageSpec{URL: "https://images.example.com/1"},
						},
					},
					Providers: v1alpha1.CrossplaneProviders{
						ImagePullSecrets: []commonapi.LocalObjectReference{
							{Name: "provider-pull-secret"},
						},
					},
				},
			},
			crossplaneInstances: []client.Object{
				&v1alpha1.Crossplane{
					ObjectMeta: metav1.ObjectMeta{Name: "xp-1", Namespace: "ns-1"},
				},
			},
			wantRequests: []ctrl.Request{
				{NamespacedName: client.ObjectKey{Name: "xp-1", Namespace: "ns-1"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build fake platform client with ProviderConfig
			platformClient := fake.NewClientBuilder().
				WithScheme(scheme.Platform).
				WithObjects(tt.providerConfig).
				Build()

			// Build fake onboarding client with Crossplane instances
			onboardingBuilder := fake.NewClientBuilder().
				WithScheme(scheme.Onboarding)
			if len(tt.crossplaneInstances) > 0 {
				onboardingBuilder = onboardingBuilder.WithObjects(tt.crossplaneInstances...)
			}
			onboardingClient := onboardingBuilder.Build()

			r := &CrossplaneReconciler{
				PlatformCluster:   clusters.NewTestClusterFromClient("platform", platformClient),
				OnboardingCluster: clusters.NewTestClusterFromClient("onboarding", onboardingClient),
				ProviderName:      TestProviderName,
			}

			got := r.mapSecretToRequests(context.Background(), tt.secret)

			if tt.wantRequests == nil {
				if got != nil {
					t.Errorf("mapSecretToRequests() = %v, want nil", got)
				}
				return
			}

			if len(got) != len(tt.wantRequests) {
				t.Fatalf("mapSecretToRequests() returned %d requests, want %d", len(got), len(tt.wantRequests))
			}
			for i, req := range got {
				if req.NamespacedName != tt.wantRequests[i].NamespacedName {
					t.Errorf("mapSecretToRequests()[%d] = %v, want %v", i, req.NamespacedName, tt.wantRequests[i].NamespacedName)
				}
			}
		})
	}
}

func Test_mapSecretToRequests_providerConfigNotFound(t *testing.T) {
	// Platform client with no ProviderConfig — simulates it not existing
	platformClient := fake.NewClientBuilder().
		WithScheme(scheme.Platform).
		Build()
	onboardingClient := fake.NewClientBuilder().
		WithScheme(scheme.Onboarding).
		Build()

	r := &CrossplaneReconciler{
		PlatformCluster:   clusters.NewTestClusterFromClient("platform", platformClient),
		OnboardingCluster: clusters.NewTestClusterFromClient("onboarding", onboardingClient),
		ProviderName:      TestProviderName,
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-secret",
			Namespace: "sp-namespace",
		},
	}

	got := r.mapSecretToRequests(context.Background(), secret)
	if got != nil {
		t.Errorf("mapSecretToRequests() = %v, want nil when ProviderConfig not found", got)
	}
}

func Test_prefixSecretName(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"short name", "privateregcred"},
		{"long name truncated", strings.Repeat("a", 60)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := prefixSecretName(tt.input)
			require.NoError(t, err)
			assert.True(t, strings.HasPrefix(got, secretNamePrefix))
			assert.LessOrEqual(t, len(got), 63)
		})
	}
}
