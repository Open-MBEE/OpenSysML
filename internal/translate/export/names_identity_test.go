package export

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/naming"
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
		segment := naming.Segment(test.name)
		if got := identityName(segment); got != test.name {
			t.Errorf("identityName(naming.Segment(%q)) = %q", test.name, got)
		}
		segments := naming.Segments(segment + "::x")
		if len(segments) != 2 || segments[1] != "x" {
			t.Errorf("naming.Segments(%q) = %q, want a name and x", segment+"::x", segments)
		}
	}
	if got := naming.Segment("Vehicle"); got != "Vehicle" {
		t.Errorf("naming.Segment(Vehicle) = %q, want it unquoted", got)
	}
	names := []string{"@2", "x", "a"}
	for _, test := range tests {
		names = append(names, test.name)
	}
	identities := map[string]string{}
	for _, name := range names {
		identity := naming.Segment(name)
		if previous, ok := identities[identity]; ok && previous != name {
			t.Errorf("naming.Segment(%q) and naming.Segment(%q) both equal %q", name, previous, identity)
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
		{"", []string{""}, []string{""}},
		{"::", []string{"", ""}, []string{"", ""}},
		{"P::", []string{"P", ""}, []string{"P", ""}},
		{"::P", []string{"", "P"}, []string{"", "P"}},
		{"P::'unfinished::@2", []string{"P", "'unfinished", "@2"}, []string{"P", "'unfinished", "@2"}},
		{"P::'closed'next::@2", []string{"P", "'closed'next", "@2"}, []string{"P", "'closed'next", "@2"}},
		{"'A::B'::X", []string{"'A::B'", "X"}, []string{"A::B", "X"}},
		{"P::'@2'::@0", []string{"P", "'@2'", "@0"}, []string{"P", "@2", "@0"}},
	} {
		segments := naming.Segments(test.qname)
		if !reflect.DeepEqual(segments, test.segments) {
			t.Errorf("naming.Segments(%q) = %q, want %q", test.qname, segments, test.segments)
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
