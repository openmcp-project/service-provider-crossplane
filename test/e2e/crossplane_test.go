package e2e

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	mcpName              = "test-mcp"
	crossplaneNamespace  = "crossplane-system"
	crossplaneDeployment = "crossplane"
	providerBTPName      = "provider-btp"
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
					if err := wait.For(openmcpconditions.Match(&obj, onboardingConfig, "Reconciled", corev1.ConditionTrue), wait.WithTimeout(5*time.Minute)); err != nil {
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
		Teardown(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			onboardingConfig, err := clusterutils.OnboardingConfig()
			if err != nil {
				t.Error(err)
				return ctx
			}
			for _, obj := range onboardingList.Items {
				if err := resources.DeleteObject(ctx, onboardingConfig, &obj, wait.WithTimeout(2*time.Minute)); err != nil {
					t.Errorf("failed to delete onboarding object: %v", err)
				}
			}
			return ctx
		}).
		Teardown(providers.DeleteMCP(mcpName, wait.WithTimeout(5*time.Minute)))

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
		if err := wait.For(conditions.New(mcp.Client().Resources()).ResourcesFound(nsList), wait.WithTimeout(3*time.Minute)); err != nil {
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
		if err := wait.For(conditions.New(mcp.Client().Resources()).ResourcesFound(deployList), wait.WithTimeout(5*time.Minute)); err != nil {
			t.Errorf("crossplane deployment not found on MCP %s: %v", mcpName, err)
			return ctx
		}
		deploy := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: crossplaneDeployment, Namespace: crossplaneNamespace},
		}
		if err := wait.For(conditions.New(mcp.Client().Resources()).DeploymentConditionMatch(deploy, appsv1.DeploymentAvailable, corev1.ConditionTrue), wait.WithTimeout(5*time.Minute)); err != nil {
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
		if err := wait.For(openmcpconditions.Match(provider, mcp, "Healthy", corev1.ConditionTrue), wait.WithTimeout(5*time.Minute)); err != nil {
			t.Errorf("crossplane provider %s not healthy on MCP %s: %v", providerName, mcpName, err)
		}
		return ctx
	}
}
