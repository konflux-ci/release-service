package helpers

import (
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"testing"
)

var (
	testObject      = &tektonv1beta1.PipelineRun{}
	testAnnotations = map[string]string{
		"a.test/annotation":              "foo",
		"another.test/annotation-string": "bar",
	}
	testLabels = map[string]string{
		"a.test/label":              "foo",
		"another.test/label-string": "bar",
	}
)

func init() {
	testObject.SetAnnotations(testAnnotations)
	testObject.SetLabels(testLabels)
}

func TestHasAnnotation(t *testing.T) {
	var tests = []struct {
		annotation string
		want       bool
	}{
		{"a.test/annotation", true},
		{"bar", false},
	}

	for _, tt := range tests {
		if tt.want != HasAnnotation(testObject, tt.annotation) {
			t.Errorf("Expected search for annotation '%s' using HasAnnotation() to return %t, returned %t", tt.annotation, tt.want, !tt.want)
		}
	}
}

func TestHasAnnotationWithValue(t *testing.T) {
	var tests = []struct {
		annotation string
		value      string
		want       bool
	}{
		{"a.test/annotation", "foo", true},
		{"a.test/annotation", "bar", false},
		{"a.missing/annotation", "", false},
	}

	for _, tt := range tests {
		if tt.want != HasAnnotationWithValue(testObject, tt.annotation, tt.value) {
			t.Errorf("Expected search for value '%s' of annotation '%s' using HasAnnotationWithValue() to return %t, returned %t", tt.value, tt.annotation, tt.want, !tt.want)
		}
	}
}

func TestHasLabel(t *testing.T) {
	var tests = []struct {
		label string
		want  bool
	}{
		{"a.test/label", true},
		{"bar", false},
	}

	for _, tt := range tests {
		if tt.want != HasLabel(testObject, tt.label) {
			t.Errorf("Expected search for label '%s' using HasLabel() to return %t, returned %t", tt.label, tt.want, !tt.want)
		}
	}
}

func TestHasLabelWithValue(t *testing.T) {
	var tests = []struct {
		label string
		value string
		want  bool
	}{
		{"a.test/label", "foo", true},
		{"a.test/label", "bar", false},
		{"a.missing/label", "", false},
	}

	for _, tt := range tests {
		if tt.want != HasLabelWithValue(testObject, tt.label, tt.value) {
			t.Errorf("Expected search for value '%s' of label '%s' using HasLabelWithValue() to return %t, returned %t", tt.value, tt.label, tt.want, !tt.want)
		}
	}
}
