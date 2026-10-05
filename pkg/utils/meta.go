//nolint:revive
package utils

import (
	"github.com/openmcp-project/control-plane-operator/pkg/juggler"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	labelManagedBy = "app.kubernetes.io/managed-by"
	// LabelManagedByValue is the value used for the "app.kubernetes.io/managed-by" label of this Service Provider.
	LabelManagedByValue = "service-provider-crossplane"
	// LabelComponentName is the label used to identify components added by the Service Provider in an MCP.
	LabelComponentName = "services.openmcp.cloud/component"
	// AnnotationManagedPI is the annotation used to put poll interval under service provider management
	AnnotationManagedPI = "open-control-plane.io/managed-poll-interval"
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
	SetLabel(obj, labelManagedBy, LabelManagedByValue)
}

// IsManaged returns a client.MatchingLabels that matches objects managed by this Service Provider.
func IsManaged() client.MatchingLabels {
	return client.MatchingLabels{labelManagedBy: LabelManagedByValue}
}

// SetAnnotation sets an annotation on the given object.
func SetAnnotation(obj v1.Object, annotation string, value string) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[annotation] = value
	obj.SetAnnotations(annotations)
}

// SetPIManaged sets the open-control-plane.io/managed-poll-interval annotation to true if it does not already exist.
func SetPIManaged(obj v1.Object) {
	a := obj.GetAnnotations()
	_, ok := a[AnnotationManagedPI]
	if !ok {
		SetAnnotation(obj, AnnotationManagedPI, "true")
	}
}

// IsPIManaged returns if AnnotationManagedPI is set to true.
func IsPIManaged(obj v1.Object) bool {
	a := obj.GetAnnotations()
	val, ok := a[AnnotationManagedPI]
	return ok && val == "true"
}

// HasComponentLabel returns a client.ListOption that matches objects with the component label.
func HasComponentLabel() client.ListOption {
	return client.HasLabels{LabelComponentName}
}

// LabelFunc sets the `managedBy` label to the passed in `managedByValue`
// and the `component` label to the name of the component the function is called with.
func LabelFunc(managedByValue string) juggler.LabelFunc {
	return func(comp juggler.Component) map[string]string {
		return map[string]string{
			labelManagedBy:     managedByValue,
			LabelComponentName: comp.GetName(),
		}
	}
}
