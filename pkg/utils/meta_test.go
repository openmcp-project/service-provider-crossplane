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
			want: map[string]string{labelManagedBy: LabelManagedByValue},
		},
		{
			name: "update existing label",
			obj: &v1alpha1.Crossplane{
				ObjectMeta: v1.ObjectMeta{
					Labels: map[string]string{"app.kubernetes.io/managed-by": "foo"},
				},
			},
			want: map[string]string{labelManagedBy: LabelManagedByValue},
		},
		{
			name: "add a second label to object",
			obj:  &v1alpha1.Crossplane{ObjectMeta: v1.ObjectMeta{Labels: map[string]string{"foo": "bar"}}},
			want: map[string]string{"foo": "bar", labelManagedBy: LabelManagedByValue},
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
	assert.DeepEqual(t, got, client.MatchingLabels{labelManagedBy: LabelManagedByValue})
}

func TestHasComponentLabel(t *testing.T) {
	got := HasComponentLabel()
	assert.DeepEqual(t, got, client.HasLabels{LabelComponentName})
}

func TestSetAnnotation(t *testing.T) {
	tests := []struct {
		name       string
		obj        v1.Object
		annotation string
		value      string
		want       map[string]string
	}{
		{
			name:       "add new annotation to object",
			obj:        &v1alpha1.Crossplane{},
			annotation: "foo",
			value:      "bar",
			want:       map[string]string{"foo": "bar"},
		},
		{
			name:       "update existing annotation",
			obj:        &v1alpha1.Crossplane{ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{"foo": "bar"}}},
			annotation: "foo",
			value:      "baz",
			want:       map[string]string{"foo": "baz"},
		},
		{
			name:       "add a second annotation to object",
			obj:        &v1alpha1.Crossplane{ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{"foo": "bar"}}},
			annotation: "foo2",
			value:      "bar2",
			want:       map[string]string{"foo": "bar", "foo2": "bar2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetAnnotation(tt.obj, tt.annotation, tt.value)
			assert.DeepEqual(t, tt.obj.GetAnnotations(), tt.want)
		})
	}
}

func TestSetPIManaged(t *testing.T) {
	tests := []struct {
		name string
		obj  v1.Object
		want map[string]string
	}{
		{
			name: "set managed by annotation",
			obj:  &v1alpha1.Crossplane{},
			want: map[string]string{AnnotationManagedPI: "true"},
		},
		{
			name: "does not update existing annotation",
			obj: &v1alpha1.Crossplane{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{AnnotationManagedPI: "false"},
				},
			},
			want: map[string]string{AnnotationManagedPI: "false"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetPIManaged(tt.obj)
			assert.DeepEqual(t, tt.obj.GetAnnotations(), tt.want)
		})
	}
}

func TestIsPIManaged(t *testing.T) {
	tests := []struct {
		name string
		obj  v1.Object
		want bool
	}{
		{
			name: "annotation set to true returns true",
			obj: &v1alpha1.Crossplane{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{AnnotationManagedPI: "true"},
				},
			},
			want: true,
		},
		{
			name: "annotation not set to true returns false",
			obj: &v1alpha1.Crossplane{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{AnnotationManagedPI: "false"},
				},
			},
			want: false,
		},
		{
			name: "annotation not set returns false",
			obj:  &v1alpha1.Crossplane{},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			assert.Equal(t, IsPIManaged(tt.obj), tt.want)
		})
	}
}
