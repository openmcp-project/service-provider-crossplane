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
)

var (
	secretHealthy = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-healthy",
		},
	}
	secretUnhealthy = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-unhealthy",
			DeletionTimestamp: ptr.To(metav1.Now()),
		},
	}
	secretA = &corev1.Secret{
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
	secretB = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-b",
		},
		Type: corev1.SecretTypeBasicAuth,
		Data: map[string][]byte{
			corev1.BasicAuthUsernameKey: []byte("admin"),
			corev1.BasicAuthPasswordKey: []byte("very_Secure"),
		},
	}
	sourceSecret = &corev1.Secret{
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

func Test_Secret(t *testing.T) {
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
			source:  client.ObjectKeyFromObject(sourceSecret),
			target:  client.ObjectKeyFromObject(secretA),
			validationFuncs: []validationFunc{
				hasName("SecretTestA"),
				isEnabled(false),
			},
		},
		{
			desc:    "should be enabled",
			enabled: true,
			source:  client.ObjectKeyFromObject(sourceSecret),
			target:  client.ObjectKeyFromObject(secretA),
			validationFuncs: []validationFunc{
				hasName("SecretTestA"),
				isEnabled(true),
				isAllowed(true),
				hasDependencies(1),
				hasNoHooks(),
				isTargetComponent(
					hasNamespace(CrossplaneNamespace),
				),
				isObjectComponent(
					objectIsType(&corev1.Secret{}),
					canCheckHealthiness(secretUnhealthy, juggler.ResourceHealthiness{
						Healthy: false,
					}),
					canCheckHealthiness(secretHealthy, juggler.ResourceHealthiness{
						Healthy: true,
					}),
					canBuildAndReconcile(nil),
					implementsOrphanedObjectsDetector(
						listTypeIs(&corev1.SecretList{}),
						hasFilterCriteria(2),
						canConvert(&corev1.SecretList{Items: []corev1.Secret{*secretA}}, 1),
						canCheckSame(&Secret{Target: client.ObjectKeyFromObject(secretA)}, &Secret{Target: client.ObjectKeyFromObject(secretA)}, true),
						canCheckSame(&Secret{Target: client.ObjectKeyFromObject(secretA)}, &Secret{Target: client.ObjectKeyFromObject(secretB)}, false),
					),
				),
			},
		},
		{
			desc:    "should fail when client returns error",
			enabled: true,
			source:  client.ObjectKeyFromObject(sourceSecret),
			target:  client.ObjectKeyFromObject(secretA),
			interceptorFuncs: interceptor.Funcs{
				Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					return errFake
				},
			},
			validationFuncs: []validationFunc{
				hasName("SecretTestA"),
				isEnabled(true),
				isAllowed(true),
				hasDependencies(1),
				hasNoHooks(),
				isTargetComponent(
					hasNamespace(CrossplaneNamespace),
				),
				isObjectComponent(
					canBuildAndReconcile(errFake),
				),
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			ctx := newContext(nil)
			fakeClient := fake.NewClientBuilder().WithInterceptorFuncs(tC.interceptorFuncs).WithObjects(sourceSecret).Build()
			c := &Secret{
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
