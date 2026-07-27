package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/openmcp-project/service-provider-crossplane/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubewait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/openmcp-project/openmcp-testing/pkg/clusterutils"
	openmcpconditions "github.com/openmcp-project/openmcp-testing/pkg/conditions"
	"github.com/openmcp-project/openmcp-testing/pkg/providers"
	"github.com/openmcp-project/openmcp-testing/pkg/resources"
)

const (
	mcpName                       = "test-mcp"
	crossplaneNamespace           = "crossplane-system"
	crossplaneDeployment          = "crossplane"
	providerBTPName               = "provider-btp"
	functionPatchAndTransformName = "function-patch-and-transform"
	timeout                       = 5 * time.Minute
)

func TestServiceProvider(t *testing.T) {
	var onboardingList unstructured.UnstructuredList

	basicProviderTest := features.New("provider test").
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			if _, err := resources.CreateObjectsFromDir(ctx, c, "platform"); err != nil {
				t.Errorf("failed to create platform cluster objects: %v", err)
			}
			return ctx
		}).
		Setup(providers.CreateMCP(mcpName)).
		Assess("verify crossplane service can be successfully consumed",
			func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
				onboardingConfig, err := clusterutils.OnboardingConfig()
				if err != nil {
					t.Error(err)
					return ctx
				}
				objList, err := resources.CreateObjectsFromDir(ctx, onboardingConfig, "onboarding")
				if err != nil {
					t.Errorf("failed to create onboarding cluster objects: %v", err)
					return ctx
				}
				for _, obj := range objList.Items {
					if err := wait.For(openmcpconditions.Match(&obj, onboardingConfig, "Reconciled", corev1.ConditionTrue), wait.WithTimeout(timeout)); err != nil {
						t.Error(err)
					}
				}
				objList.DeepCopyInto(&onboardingList)
				return ctx
			},
		).
		Assess("ManagedControlPlane: crossplane namespace exists",
			crossplaneNamespaceExists(mcpName),
		).
		Assess("ManagedControlPlane: crossplane deployment is available",
			crossplaneDeploymentReady(mcpName),
		).
		Assess("ManagedControlPlane: provider-btp is installed and healthy",
			crossplaneProviderHealthy(mcpName, providerBTPName),
		).
		Assess("ControlPlane: function-patch-and-transform is installed and healthy",
			crossplaneFunctionHealthy(mcpName, functionPatchAndTransformName),
		).
		Teardown(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			onboardingConfig, err := clusterutils.OnboardingConfig()
			if err != nil {
				t.Error(err)
				return ctx
			}
			for _, obj := range onboardingList.Items {
				if err := resources.DeleteObject(ctx, onboardingConfig, &obj, wait.WithTimeout(timeout)); err != nil {
					t.Errorf("failed to delete onboarding object: %v", err)
				}
			}
			return ctx
		}).
		Teardown(providers.DeleteMCP(mcpName, wait.WithTimeout(timeout)))

	testenv.Test(t, basicProviderTest.Feature())
}

func crossplaneNamespaceExists(mcpName string) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		mcp, err := clusterutils.MCPConfig(ctx, c, mcpName)
		if err != nil {
			t.Error(err)
			return ctx
		}
		nsList := &corev1.NamespaceList{
			Items: []corev1.Namespace{
				{ObjectMeta: metav1.ObjectMeta{Name: crossplaneNamespace}},
			},
		}
		if err := wait.For(conditions.New(mcp.Client().Resources()).ResourcesFound(nsList), wait.WithTimeout(timeout)); err != nil {
			t.Errorf("%s namespace not found on MCP %s: %v", crossplaneNamespace, mcpName, err)
		}
		return ctx
	}
}

func crossplaneDeploymentReady(mcpName string) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		mcp, err := clusterutils.MCPConfig(ctx, c, mcpName)
		if err != nil {
			t.Error(err)
			return ctx
		}
		deployList := &appsv1.DeploymentList{
			Items: []appsv1.Deployment{
				{ObjectMeta: metav1.ObjectMeta{Name: crossplaneDeployment, Namespace: crossplaneNamespace}},
			},
		}
		if err := wait.For(conditions.New(mcp.Client().Resources()).ResourcesFound(deployList), wait.WithTimeout(timeout)); err != nil {
			t.Errorf("crossplane deployment not found on MCP %s: %v", mcpName, err)
			return ctx
		}
		deploy := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: crossplaneDeployment, Namespace: crossplaneNamespace},
		}
		if err := wait.For(conditions.New(mcp.Client().Resources()).DeploymentConditionMatch(deploy, appsv1.DeploymentAvailable, corev1.ConditionTrue), wait.WithTimeout(timeout)); err != nil {
			t.Errorf("crossplane deployment not available on MCP %s: %v", mcpName, err)
		}
		return ctx
	}
}

func crossplaneProviderHealthy(mcpName, providerName string) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		mcp, err := clusterutils.MCPConfig(ctx, c, mcpName)
		if err != nil {
			t.Error(err)
			return ctx
		}
		provider := &unstructured.Unstructured{}
		provider.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "pkg.crossplane.io",
			Version: "v1",
			Kind:    "Provider",
		})
		provider.SetName(providerName)
		if err := wait.For(openmcpconditions.Match(provider, mcp, "Healthy", corev1.ConditionTrue), wait.WithTimeout(timeout)); err != nil {
			t.Errorf("crossplane provider %s not healthy on MCP %s: %v", providerName, mcpName, err)
		}
		return ctx
	}
}

func crossplaneFunctionHealthy(mcpName, functionName string) features.Func {
	return func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
		mcp, err := clusterutils.MCPConfig(ctx, c, mcpName)
		if err != nil {
			t.Error(err)
			return ctx
		}
		function := &unstructured.Unstructured{}
		function.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "pkg.crossplane.io",
			Version: "v1",
			Kind:    "Function",
		})
		function.SetName(functionName)
		if err := wait.For(openmcpconditions.Match(function, mcp, "Healthy", corev1.ConditionTrue), wait.WithTimeout(5*time.Minute)); err != nil {
			t.Errorf("crossplane function %s not healthy on ControlPlane %s: %v", functionName, mcpName, err)
		}
		return ctx
	}
}

func TestInvalidVersion(t *testing.T) {
	invalidVersion := "9.99.99"

	invalidVersionTest := features.New("invalid version test").
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			if _, err := resources.CreateObjectsFromDir(ctx, c, "platform"); err != nil {
				t.Errorf("failed to create platform cluster objects: %v", err)
			}
			return ctx
		}).
		Setup(providers.CreateMCP(mcpName)).
		Assess("verify error message contains available versions",
			func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
				onboardingConfig, err := clusterutils.OnboardingConfig()
				if err != nil {
					t.Error(err)
					return ctx
				}
				obj := &unstructured.Unstructured{}
				obj.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   v1alpha1.GroupVersion.Group,
					Version: v1alpha1.GroupVersion.Version,
					Kind:    "Crossplane",
				})
				obj.SetName(mcpName)
				obj.SetNamespace("default")
				_ = unstructured.SetNestedField(obj.Object, invalidVersion, "spec", "version")

				if err := onboardingConfig.Client().Resources().Create(ctx, obj); err != nil {
					t.Errorf("failed to create Crossplane resource: %v", err)
					return ctx
				}

				if err := wait.For(
					conditionWithMessageContains(obj, onboardingConfig, "Reconciled", corev1.ConditionFalse, invalidVersion),
					wait.WithTimeout(3*time.Minute),
				); err != nil {
					t.Errorf("expected Reconciled=False with available versions in message: %v", err)
				}

				if err := resources.DeleteObject(ctx, onboardingConfig, obj, wait.WithTimeout(2*time.Minute)); err != nil {
					t.Errorf("failed to delete Crossplane resource: %v", err)
				}
				return ctx
			},
		).
		Teardown(providers.DeleteMCP(mcpName, wait.WithTimeout(timeout)))

	testenv.Test(t, invalidVersionTest.Feature())
}

func TestInvalidProviderVersion(t *testing.T) {
	invalidProviderVersion := "v0.0.1"

	invalidProviderVersionTest := features.New("invalid provider version test").
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			if _, err := resources.CreateObjectsFromDir(ctx, c, "platform"); err != nil {
				t.Errorf("failed to create platform cluster objects: %v", err)
			}
			return ctx
		}).
		Setup(providers.CreateMCP(mcpName)).
		Assess("verify error message contains invalid provider version",
			func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
				onboardingConfig, err := clusterutils.OnboardingConfig()
				if err != nil {
					t.Error(err)
					return ctx
				}
				obj := &unstructured.Unstructured{}
				obj.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   v1alpha1.GroupVersion.Group,
					Version: v1alpha1.GroupVersion.Version,
					Kind:    "Crossplane",
				})
				obj.SetName(mcpName)
				obj.SetNamespace("default")
				_ = unstructured.SetNestedField(obj.Object, "1.20.5", "spec", "version")
				_ = unstructured.SetNestedSlice(obj.Object, []interface{}{
					map[string]interface{}{
						"name":    providerBTPName,
						"version": invalidProviderVersion,
					},
				}, "spec", "providers")

				if err := onboardingConfig.Client().Resources().Create(ctx, obj); err != nil {
					t.Errorf("failed to create Crossplane resource: %v", err)
					return ctx
				}

				if err := wait.For(
					conditionWithMessageContains(obj, onboardingConfig, "ProviderBtpReady", corev1.ConditionFalse, invalidProviderVersion),
					wait.WithTimeout(3*time.Minute),
				); err != nil {
					t.Errorf("expected ProviderBtpReady=False with invalid provider version in message: %v", err)
				}

				if err := resources.DeleteObject(ctx, onboardingConfig, obj, wait.WithTimeout(timeout)); err != nil {
					t.Errorf("failed to delete Crossplane resource: %v", err)
				}
				return ctx
			},
		).
		Teardown(providers.DeleteMCP(mcpName, wait.WithTimeout(timeout)))

	testenv.Test(t, invalidProviderVersionTest.Feature())
}

func TestInvalidVersionRecovery(t *testing.T) {
	invalidVersion := "9.99.99"
	validVersion := "1.20.5"

	recoveryTest := features.New("invalid version recovery test").
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			if _, err := resources.CreateObjectsFromDir(ctx, c, "platform"); err != nil {
				t.Errorf("failed to create platform cluster objects: %v", err)
			}
			return ctx
		}).
		Setup(providers.CreateMCP(mcpName)).
		Assess("create with invalid version and verify error",
			func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
				onboardingConfig, err := clusterutils.OnboardingConfig()
				if err != nil {
					t.Error(err)
					return ctx
				}
				obj := &unstructured.Unstructured{}
				obj.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   v1alpha1.GroupVersion.Group,
					Version: v1alpha1.GroupVersion.Version,
					Kind:    "Crossplane",
				})
				obj.SetName(mcpName)
				obj.SetNamespace("default")
				_ = unstructured.SetNestedField(obj.Object, invalidVersion, "spec", "version")

				if err := onboardingConfig.Client().Resources().Create(ctx, obj); err != nil {
					t.Errorf("failed to create Crossplane resource: %v", err)
					return ctx
				}

				if err := wait.For(
					conditionWithMessageContains(obj, onboardingConfig, "Reconciled", corev1.ConditionFalse, invalidVersion),
					wait.WithTimeout(3*time.Minute),
				); err != nil {
					t.Errorf("expected Reconciled=False with available versions in message: %v", err)
				}
				return ctx
			},
		).
		Assess("update to valid version and verify reconciliation succeeds",
			func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
				onboardingConfig, err := clusterutils.OnboardingConfig()
				if err != nil {
					t.Error(err)
					return ctx
				}
				obj := &unstructured.Unstructured{}
				obj.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   v1alpha1.GroupVersion.Group,
					Version: v1alpha1.GroupVersion.Version,
					Kind:    "Crossplane",
				})
				obj.SetName(mcpName)
				obj.SetNamespace("default")

				if err := onboardingConfig.Client().Resources().Get(ctx, obj.GetName(), obj.GetNamespace(), obj); err != nil {
					t.Errorf("failed to get Crossplane resource: %v", err)
					return ctx
				}
				_ = unstructured.SetNestedField(obj.Object, validVersion, "spec", "version")
				if err := onboardingConfig.Client().Resources().Update(ctx, obj); err != nil {
					t.Errorf("failed to update Crossplane resource: %v", err)
					return ctx
				}

				if err := wait.For(
					openmcpconditions.Match(obj, onboardingConfig, "Reconciled", corev1.ConditionTrue),
					wait.WithTimeout(timeout),
				); err != nil {
					t.Errorf("expected Reconciled=True after updating to valid version: %v", err)
				}
				return ctx
			},
		).
		Assess("ManagedControlPlane: crossplane deployment is available",
			crossplaneDeploymentReady(mcpName),
		).
		Teardown(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			onboardingConfig, err := clusterutils.OnboardingConfig()
			if err != nil {
				t.Error(err)
				return ctx
			}
			obj := &unstructured.Unstructured{}
			obj.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   v1alpha1.GroupVersion.Group,
				Version: v1alpha1.GroupVersion.Version,
				Kind:    "Crossplane",
			})
			obj.SetName(mcpName)
			obj.SetNamespace("default")
			if err := resources.DeleteObject(ctx, onboardingConfig, obj, wait.WithTimeout(timeout)); err != nil {
				t.Errorf("failed to delete Crossplane resource: %v", err)
			}
			return ctx
		}).
		Teardown(providers.DeleteMCP(mcpName, wait.WithTimeout(timeout)))

	testenv.Test(t, recoveryTest.Feature())
}

func TestInvalidProviderVersionRecovery(t *testing.T) {
	invalidProviderVersion := "v0.0.1"
	validProviderVersion := "v1.9.0"

	recoveryTest := features.New("invalid provider version recovery test").
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			if _, err := resources.CreateObjectsFromDir(ctx, c, "platform"); err != nil {
				t.Errorf("failed to create platform cluster objects: %v", err)
			}
			return ctx
		}).
		Setup(providers.CreateMCP(mcpName)).
		Assess("create with invalid provider version and verify error",
			func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
				onboardingConfig, err := clusterutils.OnboardingConfig()
				if err != nil {
					t.Error(err)
					return ctx
				}
				obj := &unstructured.Unstructured{}
				obj.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   v1alpha1.GroupVersion.Group,
					Version: v1alpha1.GroupVersion.Version,
					Kind:    "Crossplane",
				})
				obj.SetName(mcpName)
				obj.SetNamespace("default")
				_ = unstructured.SetNestedField(obj.Object, "1.20.5", "spec", "version")
				_ = unstructured.SetNestedSlice(obj.Object, []interface{}{
					map[string]interface{}{
						"name":    providerBTPName,
						"version": invalidProviderVersion,
					},
				}, "spec", "providers")

				if err := onboardingConfig.Client().Resources().Create(ctx, obj); err != nil {
					t.Errorf("failed to create Crossplane resource: %v", err)
					return ctx
				}

				if err := wait.For(
					conditionWithMessageContains(obj, onboardingConfig, "ProviderBtpReady", corev1.ConditionFalse, invalidProviderVersion),
					wait.WithTimeout(3*time.Minute),
				); err != nil {
					t.Errorf("expected ProviderBtpReady=False with invalid provider version in message: %v", err)
				}
				return ctx
			},
		).
		Assess("update to valid provider version and verify reconciliation succeeds",
			func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
				onboardingConfig, err := clusterutils.OnboardingConfig()
				if err != nil {
					t.Error(err)
					return ctx
				}
				obj := &unstructured.Unstructured{}
				obj.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   v1alpha1.GroupVersion.Group,
					Version: v1alpha1.GroupVersion.Version,
					Kind:    "Crossplane",
				})
				obj.SetName(mcpName)
				obj.SetNamespace("default")

				if err := onboardingConfig.Client().Resources().Get(ctx, obj.GetName(), obj.GetNamespace(), obj); err != nil {
					t.Errorf("failed to get Crossplane resource: %v", err)
					return ctx
				}
				_ = unstructured.SetNestedSlice(obj.Object, []interface{}{
					map[string]interface{}{
						"name":    providerBTPName,
						"version": validProviderVersion,
					},
				}, "spec", "providers")
				if err := onboardingConfig.Client().Resources().Update(ctx, obj); err != nil {
					t.Errorf("failed to update Crossplane resource: %v", err)
					return ctx
				}

				if err := wait.For(
					openmcpconditions.Match(obj, onboardingConfig, "Reconciled", corev1.ConditionTrue),
					wait.WithTimeout(timeout),
				); err != nil {
					t.Errorf("expected Reconciled=True after updating to valid provider version: %v", err)
				}
				return ctx
			},
		).
		Assess("ManagedControlPlane: provider-btp is installed and healthy",
			crossplaneProviderHealthy(mcpName, providerBTPName),
		).
		Teardown(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			onboardingConfig, err := clusterutils.OnboardingConfig()
			if err != nil {
				t.Error(err)
				return ctx
			}
			obj := &unstructured.Unstructured{}
			obj.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   v1alpha1.GroupVersion.Group,
				Version: v1alpha1.GroupVersion.Version,
				Kind:    "Crossplane",
			})
			obj.SetName(mcpName)
			obj.SetNamespace("default")
			if err := resources.DeleteObject(ctx, onboardingConfig, obj, wait.WithTimeout(timeout)); err != nil {
				t.Errorf("failed to delete Crossplane resource: %v", err)
			}
			return ctx
		}).
		Teardown(providers.DeleteMCP(mcpName, wait.WithTimeout(timeout)))

	testenv.Test(t, recoveryTest.Feature())
}

func conditionWithMessageContains(obj *unstructured.Unstructured, cfg *envconf.Config, conditionType string, conditionStatus corev1.ConditionStatus, substring string) kubewait.ConditionWithContextFunc {
	return func(ctx context.Context) (bool, error) {
		klog.Infof("waiting for condition %s=%s with message containing %q", conditionType, conditionStatus, substring)
		if err := cfg.Client().Resources().Get(ctx, obj.GetName(), obj.GetNamespace(), obj); err != nil {
			return false, nil
		}
		conditionsSlice, ok, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
		if err != nil || !ok {
			return false, nil
		}
		for _, c := range conditionsSlice {
			cond, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			if cond["type"] != conditionType {
				continue
			}
			status, _ := cond["status"].(string)
			message, _ := cond["message"].(string)
			klog.Infof("condition %s: status=%s, message=%s", conditionType, status, message)
			if status == string(conditionStatus) && strings.Contains(message, substring) {
				return true, nil
			}
		}
		return false, nil
	}
}
