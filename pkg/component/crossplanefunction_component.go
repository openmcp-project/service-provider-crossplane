package component

import (
	"context"
	"fmt"
	"strings"

	crossplanev1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openmcp-project/control-plane-operator/pkg/controlplane/components"
	"github.com/openmcp-project/control-plane-operator/pkg/juggler"
	"github.com/openmcp-project/control-plane-operator/pkg/juggler/object"
	"github.com/openmcp-project/control-plane-operator/pkg/utils/rcontext"

	"github.com/openmcp-project/service-provider-crossplane/api/v1alpha1"
	"github.com/openmcp-project/service-provider-crossplane/pkg/crossplane"
	"github.com/openmcp-project/service-provider-crossplane/pkg/utils"
)

var _ object.ObjectComponent = &CrossplaneFunction{}
var _ object.OrphanedObjectsDetector = &CrossplaneFunction{}
var _ components.TargetComponent = &CrossplaneFunction{}

// CrossplaneFunction represents a Crossplane function configuration for a Crossplane instance.
type CrossplaneFunction struct {
	Config      *v1alpha1.CrossplaneFunctionConfig
	Enabled     bool
	PullSecrets []corev1.LocalObjectReference
}

// BuildObjectToReconcile implements object.ObjectComponent.
func (c *CrossplaneFunction) BuildObjectToReconcile(_ context.Context) (client.Object, types.NamespacedName, error) {
	obj, key := crossplane.EmptyFunctionFromConfig(*c.Config)
	return obj, key, nil
}

// ReconcileObject implements object.ObjectComponent.
func (c *CrossplaneFunction) ReconcileObject(ctx context.Context, obj client.Object) error {
	versionResolveFn := rcontext.VersionResolver(ctx)

	comp, err := versionResolveFn(crossplane.FunctionNameForFunctionConfig(c.Config), c.Config.Version)
	if err != nil {
		return err
	}

	objFunction := obj.(*crossplanev1.Function)
	utils.SetManagedBy(objFunction)
	objFunction.Spec.Package = comp.DockerRef
	objFunction.Spec.PackagePullPolicy = ptr.To(corev1.PullIfNotPresent)
	objFunction.Spec.PackagePullSecrets = c.PullSecrets
	return nil
}

// OrphanDetectorContext implements object.OrphanedObjectsDetector.
func (*CrossplaneFunction) OrphanDetectorContext(_ context.Context) object.DetectorContext {
	return object.DetectorContext{
		ListType: &crossplanev1.FunctionList{},
		FilterCriteria: object.FilterCriteria{
			utils.IsManaged(),
			utils.HasComponentLabel(),
		},
		ConvertFunc: func(list client.ObjectList) []juggler.Component {
			functions := []juggler.Component{} //nolint:prealloc
			for _, function := range (list.(*crossplanev1.FunctionList)).Items {
				cf := &CrossplaneFunction{Config: &v1alpha1.CrossplaneFunctionConfig{
					Name: function.Name,
				}}
				functions = append(functions, cf)
			}
			return functions
		},
		SameFunc: func(configured, detected juggler.Component) bool {
			configuredF := configured.(*CrossplaneFunction)
			detectedF := detected.(*CrossplaneFunction)
			return configuredF.Config.Name == detectedF.Config.Name
		},
	}
}

// IsObjectHealthy implements object.ObjectComponent.
func (c *CrossplaneFunction) IsObjectHealthy(obj client.Object) juggler.ResourceHealthiness {
	function := obj.(*crossplanev1.Function)

	installed := function.GetCondition(crossplanev1.TypeInstalled)
	if installed.Status != corev1.ConditionTrue {
		return juggler.ResourceHealthiness{
			Healthy: false,
			Message: fmt.Sprintf("Function installation is pending (%s). %s", installed.Reason, installed.Message),
		}
	}

	healthy := function.GetCondition(crossplanev1.TypeHealthy)
	return juggler.ResourceHealthiness{
		Healthy: healthy.Status == corev1.ConditionTrue,
		Message: fmt.Sprintf("%s: %s", healthy.Reason, healthy.Message),
	}
}

// GetNamespace implements TargetComponent.
func (c *CrossplaneFunction) GetNamespace() string {
	return CrossplaneNamespace
}

// IsInstallable implements Component.
func (c *CrossplaneFunction) IsInstallable(ctx context.Context) (bool, error) {
	rfn := rcontext.VersionResolver(ctx)
	if _, err := rfn(crossplane.FunctionNameForFunctionConfig(c.Config), c.Config.Version); err != nil {
		return false, err
	}
	return true, nil
}

// GetName implements Component.
func (c *CrossplaneFunction) GetName() string {
	return formatFunctionName(c.Config.Name)
}

// GetDependencies implements Component.
func (c *CrossplaneFunction) GetDependencies() []juggler.Component {
	return []juggler.Component{&Crossplane{}}
}

// IsEnabled implements Component.
func (c *CrossplaneFunction) IsEnabled() bool {
	return c.Enabled
}

// Hooks implements Component.
func (*CrossplaneFunction) Hooks() juggler.ComponentHooks {
	return juggler.ComponentHooks{}
}

func formatFunctionName(functionName string) string {
	parts := strings.Split(functionName, "-")
	for i, part := range parts {
		parts[i] = cases.Title(language.English).String(part)
	}
	return strings.Join(parts, "")
}
