package utils

import (
	"testing"

	"gotest.tools/v3/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openmcp-project/service-provider-crossplane/api/v1alpha1"
)

func TestSetLabel(t *testing.T) {
	tests := []struct {
		name  string
		obj   v1.Object
		label string
		value string
		want  map[string]string
	}{
		{
			name:  "add new label to object",
			obj:   &v1alpha1.Crossplane{},
			label: "foo",
			value: "bar",
			want:  map[string]string{"foo": "bar"},
		},
		{
			name:  "update existing label",
			obj:   &v1alpha1.Crossplane{ObjectMeta: v1.ObjectMeta{Labels: map[string]string{"foo": "bar"}}},
			label: "foo",
			value: "baz",
			want:  map[string]string{"foo": "baz"},
		},
		{
			name:  "add a second label to object",
			obj:   &v1alpha1.Crossplane{ObjectMeta: v1.ObjectMeta{Labels: map[string]string{"foo": "bar"}}},
			label: "abc",
			value: "xyz",
			want:  map[string]string{"foo": "bar", "abc": "xyz"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetLabel(tt.obj, tt.label, tt.value)
			assert.DeepEqual(t, tt.obj.GetLabels(), tt.want)
		})
	}
}

func TestSetManagedBy(t *testing.T) {
	tests := []struct {
		name string
		obj  v1.Object
		want map[string]string
	}{
		{
			name: "set managed by label",
			obj:  &v1alpha1.Crossplane{},
			want: map[string]string{labelManagedBy: labelManagedByValue},
		},
		{
			name: "update existing label",
			obj: &v1alpha1.Crossplane{
				ObjectMeta: v1.ObjectMeta{
					Labels: map[string]string{"app.kubernetes.io/managed-by": "foo"},
				},
			},
			want: map[string]string{labelManagedBy: labelManagedByValue},
		},
		{
			name: "add a second label to object",
			obj:  &v1alpha1.Crossplane{ObjectMeta: v1.ObjectMeta{Labels: map[string]string{"foo": "bar"}}},
			want: map[string]string{"foo": "bar", labelManagedBy: labelManagedByValue},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetManagedBy(tt.obj)
			assert.DeepEqual(t, tt.obj.GetLabels(), tt.want)
		})
	}
}

func TestIsManaged(t *testing.T) {
	got := IsManaged()
	assert.DeepEqual(t, got, client.MatchingLabels{labelManagedBy: labelManagedByValue})
}

func TestHasComponentLabel(t *testing.T) {
	got := HasComponentLabel()
	assert.DeepEqual(t, got, client.HasLabels{LabelComponentName})
}
