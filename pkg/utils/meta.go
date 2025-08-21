package utils

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	labelManagedBy      = "app.kubernetes.io/managed-by"
	labelManagedByValue = "service-provider-crossplane"

	// LabelComponentName is the label used to identify components added by the Service Provider in an MCP.
	LabelComponentName = "services.openmcp.cloud/component"
)

// SetLabel sets a label on the given object.
func SetLabel(obj v1.Object, label string, value string) {
	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[label] = value
	obj.SetLabels(labels)
}

// SetManagedBy sets the "app.kubernetes.io/managed-by" label on the given object.
func SetManagedBy(obj v1.Object) {
	SetLabel(obj, labelManagedBy, labelManagedByValue)
}

// IsManaged returns a client.MatchingLabels that matches objects managed by this Service Provider.
func IsManaged() client.MatchingLabels {
	return client.MatchingLabels{labelManagedBy: labelManagedByValue}
}

// HasComponentLabel returns a client.ListOption that matches objects with the component label.
func HasComponentLabel() client.ListOption {
	return client.HasLabels{LabelComponentName}
}
