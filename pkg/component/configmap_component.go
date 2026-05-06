package component

import (
	"context"
	"strings"

	"github.com/openmcp-project/control-plane-operator/pkg/controlplane/components"
	"github.com/openmcp-project/control-plane-operator/pkg/juggler"
	"github.com/openmcp-project/control-plane-operator/pkg/juggler/object"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openmcp-project/service-provider-crossplane/pkg/utils"
)

var _ object.ObjectComponent = &ConfigMap{}
var _ object.OrphanedObjectsDetector = &ConfigMap{}
var _ components.TargetComponent = &ConfigMap{}
var _ juggler.StatusVisibility = &ConfigMap{}

// ConfigMap represents a Kubernetes ConfigMap that is copied from a source namespace to the Crossplane namespace.
type ConfigMap struct {
	SourceClient   client.Client
	Source, Target types.NamespacedName
	Enabled        bool
}

// IsStatusInternal implements StatusVisibility interface.
func (c *ConfigMap) IsStatusInternal() bool {
	return true
}

// GetNamespace implements TargetComponent.
func (c *ConfigMap) GetNamespace() string {
	return c.Target.Namespace
}

// OrphanDetectorContext implements OrphanedObjectsDetector.
func (c *ConfigMap) OrphanDetectorContext(_ context.Context) object.DetectorContext {
	return object.DetectorContext{
		ListType: &corev1.ConfigMapList{},
		FilterCriteria: object.FilterCriteria{
			utils.IsManaged(),
			utils.HasComponentLabel(),
		},
		ConvertFunc: func(list client.ObjectList) []juggler.Component {
			var configMaps []juggler.Component //nolint:prealloc
			for _, configMap := range (list.(*corev1.ConfigMapList)).Items {
				configMaps = append(configMaps, &ConfigMap{Target: client.ObjectKeyFromObject(&configMap)})
			}
			return configMaps
		},
		SameFunc: func(configured, detected juggler.Component) bool {
			configuredC := configured.(*ConfigMap)
			detectedC := detected.(*ConfigMap)
			return configuredC.Target == detectedC.Target
		},
	}
}

// GetName implements object.ObjectComponent.
func (c *ConfigMap) GetName() string {
	return formatConfigMapName("ConfigMap", c.Target.Name)
}

// GetDependencies implements object.ObjectComponent.
func (c *ConfigMap) GetDependencies() []juggler.Component {
	return []juggler.Component{&Crossplane{}}
}

// IsEnabled implements object.ObjectComponent.
func (c *ConfigMap) IsEnabled() bool {
	return c.Enabled
}

// Hooks implements object.ObjectComponent.
func (c *ConfigMap) Hooks() juggler.ComponentHooks {
	return juggler.ComponentHooks{}
}

// IsInstallable implements object.ObjectComponent.
func (c *ConfigMap) IsInstallable(_ context.Context) (bool, error) {
	return true, nil
}

// BuildObjectToReconcile implements object.ObjectComponent.
func (c *ConfigMap) BuildObjectToReconcile(_ context.Context) (client.Object, types.NamespacedName, error) {
	return &corev1.ConfigMap{}, c.Target, nil
}

// ReconcileObject implements object.ObjectComponent.
func (c *ConfigMap) ReconcileObject(ctx context.Context, obj client.Object) error {
	sourceConfigMap := &corev1.ConfigMap{}
	// If configmap is not enabled (= should be deleted), then we don't need to get it from the API server.
	if c.Enabled {
		if err := c.SourceClient.Get(ctx, c.Source, sourceConfigMap); err != nil {
			return err
		}
	}

	objConfigMap := obj.(*corev1.ConfigMap)

	// todo: what about labels?

	objConfigMap.Data = sourceConfigMap.Data
	objConfigMap.BinaryData = sourceConfigMap.BinaryData

	return nil
}

// IsObjectHealthy implements object.ObjectComponent.
func (c *ConfigMap) IsObjectHealthy(obj client.Object) juggler.ResourceHealthiness {
	return juggler.ResourceHealthiness{
		// ConfigMap has no status field.
		Healthy: obj.GetDeletionTimestamp() == nil,
	}
}

func formatConfigMapName(prefix, name string) string {
	parts := strings.Split(name, "-")
	for i, part := range parts {
		parts[i] = cases.Title(language.English).String(part)
	}
	parts = append([]string{prefix}, parts...)
	return strings.Join(parts, "")
}
