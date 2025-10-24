//nolint:dupl,lll
package component

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/openmcp-project/control-plane-operator/pkg/constants"
	"github.com/openmcp-project/control-plane-operator/pkg/juggler"
	"github.com/openmcp-project/control-plane-operator/pkg/utils/rcontext"
)

const stableMCPNamespace = "mcp--test"

var (
	pSecretHealthy = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-healthy",
		},
	}
	pSecretUnhealthy = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-unhealthy",
			DeletionTimestamp: ptr.To(metav1.Now()),
		},
	}
	pSecretA = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-a",
			Namespace: CrossplaneNamespace,
		},
		Type: corev1.SecretTypeBasicAuth,
		Data: map[string][]byte{
			corev1.BasicAuthUsernameKey: []byte("admin"),
			corev1.BasicAuthPasswordKey: []byte("very_Secure"),
		},
	}
	pSecretB = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-b",
		},
		Type: corev1.SecretTypeBasicAuth,
		Data: map[string][]byte{
			corev1.BasicAuthUsernameKey: []byte("admin"),
			corev1.BasicAuthPasswordKey: []byte("very_Secure"),
		},
	}
	pSourceSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-source",
			Namespace: "some-namespace",
			Labels: map[string]string{
				constants.LabelCopyToCP: "true",
			},
		},
		Type: corev1.SecretTypeBasicAuth,
		Data: map[string][]byte{
			corev1.BasicAuthUsernameKey: []byte("admin"),
			corev1.BasicAuthPasswordKey: []byte("very_Secure"),
		},
	}
)

func Test_PlatformSecret(t *testing.T) {
	testCases := []struct {
		desc             string
		enabled          bool
		source, target   types.NamespacedName
		validationFuncs  []validationFunc
		interceptorFuncs interceptor.Funcs
	}{
		{
			desc:    "should be disabled",
			enabled: false,
			source:  client.ObjectKeyFromObject(pSourceSecret),
			target:  client.ObjectKeyFromObject(pSecretA),
			validationFuncs: []validationFunc{
				hasName("PlatformSecretTestA"),
				isEnabled(false),
			},
		},
		{
			desc:    "should be enabled",
			enabled: true,
			source:  client.ObjectKeyFromObject(pSourceSecret),
			target:  client.ObjectKeyFromObject(pSecretA),
			validationFuncs: []validationFunc{
				hasName("PlatformSecretTestA"),
				isEnabled(true),
				isAllowed(true),
				hasDependencies(1),
				hasNoHooks(),
				isObjectComponent(
					objectIsType(&corev1.Secret{}),
					canCheckHealthiness(pSecretUnhealthy, juggler.ResourceHealthiness{
						Healthy: false,
					}),
					canCheckHealthiness(pSecretHealthy, juggler.ResourceHealthiness{
						Healthy: true,
					}),
					canBuildAndReconcile(nil),
					implementsOrphanedObjectsDetector(
						listTypeIs(&corev1.SecretList{}),
						hasFilterCriteria(2),
						canConvert(&corev1.SecretList{Items: []corev1.Secret{*pSecretA}}, 1),
						canCheckSame(&PlatformSecret{Target: client.ObjectKeyFromObject(pSecretA)}, &PlatformSecret{Target: client.ObjectKeyFromObject(pSecretA)}, true),
						canCheckSame(&PlatformSecret{Target: client.ObjectKeyFromObject(pSecretA)}, &PlatformSecret{Target: client.ObjectKeyFromObject(pSecretB)}, false),
					),
				),
			},
		},
		{
			desc:    "should fail when client returns error",
			enabled: true,
			source:  client.ObjectKeyFromObject(pSourceSecret),
			target:  client.ObjectKeyFromObject(pSecretA),
			interceptorFuncs: interceptor.Funcs{
				Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					return errFake
				},
			},
			validationFuncs: []validationFunc{
				hasName("PlatformSecretTestA"),
				isEnabled(true),
				isAllowed(true),
				hasDependencies(1),
				hasNoHooks(),
				isObjectComponent(
					canBuildAndReconcile(errFake),
				),
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			ctx := newContext(nil)
			rcontext.WithTenantNamespace(ctx, stableMCPNamespace)
			fakeClient := fake.NewClientBuilder().WithInterceptorFuncs(tC.interceptorFuncs).WithObjects(sourceSecret).Build()
			c := &PlatformSecret{
				Enabled:      tC.enabled,
				SourceClient: fakeClient,
				Source:       tC.source,
				Target:       tC.target,
			}
			for _, vfn := range tC.validationFuncs {
				vfn(t, ctx, c)
			}
		})
	}
}
