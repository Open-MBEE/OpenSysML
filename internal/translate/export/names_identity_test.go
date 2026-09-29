package export

import (
	"reflect"
	"testing"
)

func TestIdentityNameSegments(t *testing.T) {
	tests := []struct {
		name string
	}{
		{"@2"},
		{"@02"},
		{"@"},
		{"a::b"},
		{"a'::b"},
		{`a::b\`},
		{`a\::b`},
		{`'@2'`},
		{`'x'`},
		{"x'"},
		{"'"},
		{`x\`},
		{`it\'s`},
		{`'A::B'::X`},
		{`'a'::b`},
	}
	for _, test := range tests {
		segment := identitySegment(test.name)
		if got := identityName(segment); got != test.name {
			t.Errorf("identityName(identitySegment(%q)) = %q", test.name, got)
		}
		segments := identitySegments(segment + "::x")
		if len(segments) != 2 || segments[1] != "x" {
			t.Errorf("identitySegments(%q) = %q, want a name and x", segment+"::x", segments)
		}
	}
	if got := identitySegment("Vehicle"); got != "Vehicle" {
		t.Errorf("identitySegment(Vehicle) = %q, want it unquoted", got)
	}
	names := []string{"@2", "x", "a"}
	for _, test := range tests {
		names = append(names, test.name)
	}
	identities := map[string]string{}
	for _, name := range names {
		identity := identitySegment(name)
		if previous, ok := identities[identity]; ok && previous != name {
			t.Errorf("identitySegment(%q) and identitySegment(%q) both equal %q", name, previous, identity)
		}
		identities[identity] = name
	}
	for _, test := range []struct {
		segment string
		name    string
	}{
		{`'a\qb'`, "aqb"},
		{`'unfinished`, `'unfinished`},
		{`'closed'next`, `'closed'next`},
	} {
		if got := identityName(test.segment); got != test.name {
			t.Errorf("identityName(%q) = %q, want %q", test.segment, got, test.name)
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
