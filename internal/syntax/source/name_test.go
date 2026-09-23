package source

import (
	"reflect"
	"testing"
)

func TestQualifiedNameSegments(t *testing.T) {
	for _, tc := range []struct {
		text string
		want []string
	}{
		{"A::B", []string{"A", "B"}},
		{"'Sep::Pkg'::x", []string{"Sep::Pkg", "x"}},
		{`'a\'b'::c`, []string{`a\'b`, "c"}},
		{"x", []string{"x"}},
	} {
		got, ok := QualifiedNameSegments(tc.text)
		if !ok {
			t.Errorf("QualifiedNameSegments(%q) failed", tc.text)
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("QualifiedNameSegments(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
	if _, ok := QualifiedNameSegments("A::"); ok {
		t.Error("QualifiedNameSegments(\"A::\") succeeded, want a failure")
	}
}
