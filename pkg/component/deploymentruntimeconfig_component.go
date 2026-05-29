package component

import (
	"context"
	"fmt"

	crossplanev1beta1 "github.com/crossplane/crossplane/apis/v2/pkg/v1beta1"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openmcp-project/control-plane-operator/pkg/controlplane/components"
	"github.com/openmcp-project/control-plane-operator/pkg/juggler"
	"github.com/openmcp-project/control-plane-operator/pkg/juggler/object"

	"github.com/openmcp-project/service-provider-crossplane/pkg/utils"
)

var _ object.ObjectComponent = &DeploymentRuntimeConfig{}
var _ object.OrphanedObjectsDetector = &DeploymentRuntimeConfig{}
var _ components.TargetComponent = &DeploymentRuntimeConfig{}

// DeploymentRuntimeConfig represents a Crossplane DeploymentRuntimeConfig configuration.
type DeploymentRuntimeConfig struct {
	Name    string
	Config  *crossplanev1beta1.DeploymentRuntimeConfigSpec
	Enabled bool
}

// BuildObjectToReconcile implements object.ObjectComponent.
func (d *DeploymentRuntimeConfig) BuildObjectToReconcile(_ context.Context) (client.Object, types.NamespacedName, error) {
	obj := &crossplanev1beta1.DeploymentRuntimeConfig{}
	key := types.NamespacedName{
		Name: d.Name,
	}
	return obj, key, nil
}

// ReconcileObject implements object.ObjectComponent.
func (d *DeploymentRuntimeConfig) ReconcileObject(_ context.Context, obj client.Object) error {
	drc := obj.(*crossplanev1beta1.DeploymentRuntimeConfig)
	utils.SetManagedBy(drc)

	drc.Spec = *d.Config

	return nil
}

// OrphanDetectorContext implements object.OrphanedObjectsDetector.
func (*DeploymentRuntimeConfig) OrphanDetectorContext(_ context.Context) object.DetectorContext {
	return object.DetectorContext{
		ListType: &crossplanev1beta1.DeploymentRuntimeConfigList{},
		FilterCriteria: object.FilterCriteria{
			utils.IsManaged(),
			utils.HasComponentLabel(),
		},
		ConvertFunc: func(list client.ObjectList) []juggler.Component {
			configs := []juggler.Component{} //nolint:prealloc
			for _, config := range (list.(*crossplanev1beta1.DeploymentRuntimeConfigList)).Items {
				// since we only need the name for the SameFunc, there is no need to copy the whole object
				drc := &DeploymentRuntimeConfig{
					Name:   config.Name,
					Config: &config.Spec,
				}
				configs = append(configs, drc)
			}
			return configs
		},
		SameFunc: func(configured, detected juggler.Component) bool {
			configuredDRC := configured.(*DeploymentRuntimeConfig)
			detectedDRC := detected.(*DeploymentRuntimeConfig)
			return configuredDRC.Name == detectedDRC.Name
		},
	}
}

// IsObjectHealthy implements object.ObjectComponent.
func (d *DeploymentRuntimeConfig) IsObjectHealthy(_ client.Object) juggler.ResourceHealthiness {
	// DeploymentRuntimeConfig is a configuration resource without status conditions
	// If the resource exists, it's considered healthy
	return juggler.ResourceHealthiness{
		Healthy: true,
		Message: "DeploymentRuntimeConfig is configured",
	}
}

// GetNamespace implements TargetComponent.
func (d *DeploymentRuntimeConfig) GetNamespace() string {
	return CrossplaneNamespace
}

// IsInstallable implements Component.
func (d *DeploymentRuntimeConfig) IsInstallable(_ context.Context) (bool, error) {
	// DeploymentRuntimeConfig is always installable if Crossplane is installed (handled by dependency)
	return true, nil
}

// GetName implements Component.
func (d *DeploymentRuntimeConfig) GetName() string {
	return fmt.Sprintf("DeploymentRuntimeConfig%s", cases.Title(language.AmericanEnglish).String(d.Name))
}

// GetDependencies implements Component.
func (d *DeploymentRuntimeConfig) GetDependencies() []juggler.Component {
	return []juggler.Component{&Crossplane{}}
}

// IsEnabled implements Component.
func (d *DeploymentRuntimeConfig) IsEnabled() bool {
	return d.Enabled
}

// Hooks implements Component.
func (*DeploymentRuntimeConfig) Hooks() juggler.ComponentHooks {
	return juggler.ComponentHooks{}
}
