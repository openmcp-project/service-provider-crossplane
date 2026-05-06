package component

import (
	"context"

	"github.com/openmcp-project/control-plane-operator/pkg/juggler"
	"github.com/openmcp-project/control-plane-operator/pkg/juggler/object"
	"github.com/openmcp-project/control-plane-operator/pkg/utils/rcontext"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openmcp-project/service-provider-crossplane/pkg/utils"
)

var _ object.ObjectComponent = &PlatformSecret{}
var _ object.OrphanedObjectsDetector = &PlatformSecret{}
var _ juggler.StatusVisibility = &PlatformSecret{}

// PlatformSecret represents a Kubernetes Secret that is copied from a source namespace in the Platform cluster to the ManagedControlPlane tenant namespace in the Plaform cluster.
type PlatformSecret struct {
	SourceClient   client.Client
	Source, Target types.NamespacedName
	Enabled        bool
}

// IsStatusInternal implements StatusVisibility interface.
func (s *PlatformSecret) IsStatusInternal() bool {
	return true
}

// OrphanDetectorContext implements OrphanedObjectsDetector.
func (s *PlatformSecret) OrphanDetectorContext(ctx context.Context) object.DetectorContext {
	mcpNamespaceOnPlatformCluster := rcontext.TenantNamespace(ctx)

	return object.DetectorContext{
		ListType: &corev1.SecretList{},
		FilterCriteria: object.FilterCriteria{
			utils.IsManaged(),
			utils.HasComponentLabel(),
			client.InNamespace(mcpNamespaceOnPlatformCluster),
		},
		ConvertFunc: func(list client.ObjectList) []juggler.Component {
			var secrets []juggler.Component //nolint:prealloc
			for _, secret := range (list.(*corev1.SecretList)).Items {
				secrets = append(secrets, &PlatformSecret{Target: client.ObjectKeyFromObject(&secret)})
			}
			return secrets
		},
		SameFunc: func(configured, detected juggler.Component) bool {
			configuredS := configured.(*PlatformSecret)
			detectedS := detected.(*PlatformSecret)
			return configuredS.Target == detectedS.Target
		},
	}
}

// GetName implements object.ObjectComponent.
func (s *PlatformSecret) GetName() string {
	return formatSecretName("PlatformSecret", s.Target.Name)
}

// GetDependencies implements object.ObjectComponent.
func (s *PlatformSecret) GetDependencies() []juggler.Component {
	return []juggler.Component{
		&Crossplane{},
	}
}

// IsEnabled implements object.ObjectComponent.
func (s *PlatformSecret) IsEnabled() bool {
	return s.Enabled
}

// Hooks implements object.ObjectComponent.
func (s *PlatformSecret) Hooks() juggler.ComponentHooks {
	return juggler.ComponentHooks{}
}

// IsInstallable implements object.ObjectComponent.
func (s *PlatformSecret) IsInstallable(_ context.Context) (bool, error) {
	return true, nil
}

// BuildObjectToReconcile implements object.ObjectComponent.
func (s *PlatformSecret) BuildObjectToReconcile(_ context.Context) (client.Object, types.NamespacedName, error) {
	return &corev1.Secret{}, types.NamespacedName{
		Name:      s.Target.Name,
		Namespace: s.Target.Namespace,
	}, nil
}

// ReconcileObject implements object.ObjectComponent.
func (s *PlatformSecret) ReconcileObject(ctx context.Context, obj client.Object) error {
	sourceSecret := &corev1.Secret{}
	// If secret is not enabled (= should be deleted), then we don't need to get it from the API server.
	if s.Enabled {
		if err := s.SourceClient.Get(ctx, s.Source, sourceSecret); err != nil {
			return err
		}
	}

	objSecret := obj.(*corev1.Secret)

	// todo: what about labels?

	objSecret.Type = sourceSecret.Type
	objSecret.Data = sourceSecret.Data

	return nil
}

// IsObjectHealthy implements object.ObjectComponent.
func (s *PlatformSecret) IsObjectHealthy(obj client.Object) juggler.ResourceHealthiness {
	return juggler.ResourceHealthiness{
		// Secret has no status field.
		Healthy: obj.GetDeletionTimestamp() == nil,
	}
}
