package export

import (
	"reflect"
	"testing"
)

func TestIdentityNameSegments(t *testing.T) {
	tests := []struct {
		name     string
		identity string
	}{
		{"@2", "'@2'"},
		{"@02", "@02"},
		{"@", "@"},
		{"A::B", "'A::B'"},
		{`it\'s`, `it\'s`},
	}
	for _, test := range tests {
		if got := identitySegment(test.name); got != test.identity {
			t.Errorf("identitySegment(%q) = %q, want %q", test.name, got, test.identity)
		}
	}

	for _, test := range []struct {
		qname    string
		segments []string
		names    []string
	}{
		{"'A::B'::X", []string{"'A::B'", "X"}, []string{"A::B", "X"}},
		{"P::'@2'::@0", []string{"P", "'@2'", "@0"}, []string{"P", "@2", "@0"}},
	} {
		segments := identitySegments(test.qname)
		if !reflect.DeepEqual(segments, test.segments) {
			t.Errorf("identitySegments(%q) = %q, want %q", test.qname, segments, test.segments)
		}
		names := make([]string, len(segments))
		for i, segment := range segments {
			names[i] = identityName(segment)
		}
		if !reflect.DeepEqual(names, test.names) {
			t.Errorf("identityName of %q = %q, want %q", segments, names, test.names)
		}
	}
}
