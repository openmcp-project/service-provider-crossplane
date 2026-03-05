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
	configMapHealthy = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-healthy",
		},
	}
	configMapUnhealthy = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-unhealthy",
			DeletionTimestamp: ptr.To(metav1.Now()),
		},
	}
	configMapA = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-a",
			Namespace: CrossplaneNamespace,
		},
		Data: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}
	configMapB = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-b",
		},
		Data: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}
	sourceConfigMap = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-source",
			Namespace: "some-namespace",
			Labels: map[string]string{
				constants.LabelCopyToCP: "true",
			},
		},
		Data: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
		BinaryData: map[string][]byte{
			"binary-key": []byte("binary-value"),
		},
	}
)

func Test_ConfigMap(t *testing.T) {
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
			source:  client.ObjectKeyFromObject(sourceConfigMap),
			target:  client.ObjectKeyFromObject(configMapA),
			validationFuncs: []validationFunc{
				hasName("ConfigMapTestA"),
				isEnabled(false),
			},
		},
		{
			desc:    "should be enabled",
			enabled: true,
			source:  client.ObjectKeyFromObject(sourceConfigMap),
			target:  client.ObjectKeyFromObject(configMapA),
			validationFuncs: []validationFunc{
				hasName("ConfigMapTestA"),
				isEnabled(true),
				isAllowed(true),
				hasDependencies(1),
				hasNoHooks(),
				isTargetComponent(
					hasNamespace(CrossplaneNamespace),
				),
				isObjectComponent(
					objectIsType(&corev1.ConfigMap{}),
					canCheckHealthiness(configMapUnhealthy, juggler.ResourceHealthiness{
						Healthy: false,
					}),
					canCheckHealthiness(configMapHealthy, juggler.ResourceHealthiness{
						Healthy: true,
					}),
					canBuildAndReconcile(nil),
					implementsOrphanedObjectsDetector(
						listTypeIs(&corev1.ConfigMapList{}),
						hasFilterCriteria(2),
						canConvert(&corev1.ConfigMapList{Items: []corev1.ConfigMap{*configMapA}}, 1),
						canCheckSame(&ConfigMap{Target: client.ObjectKeyFromObject(configMapA)}, &ConfigMap{Target: client.ObjectKeyFromObject(configMapA)}, true),
						canCheckSame(&ConfigMap{Target: client.ObjectKeyFromObject(configMapA)}, &ConfigMap{Target: client.ObjectKeyFromObject(configMapB)}, false),
					),
				),
			},
		},
		{
			desc:    "should fail when client returns error",
			enabled: true,
			source:  client.ObjectKeyFromObject(sourceConfigMap),
			target:  client.ObjectKeyFromObject(configMapA),
			interceptorFuncs: interceptor.Funcs{
				Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					return errFake
				},
			},
			validationFuncs: []validationFunc{
				hasName("ConfigMapTestA"),
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
			fakeClient := fake.NewClientBuilder().WithInterceptorFuncs(tC.interceptorFuncs).WithObjects(sourceConfigMap).Build()
			c := &ConfigMap{
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
