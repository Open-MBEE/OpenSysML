package project

import (
	"errors"
	"testing"
)

func TestVersionRangeMatches(t *testing.T) {
	cases := []struct {
		constraint string
		yes, no    []string
	}{
		{"", []string{"0.0.1", "1.0.0", "v9.9.9", "1.0.0-rc.1"}, nil},
		{"*", []string{"0.0.1", "2.0.0"}, nil},
		{"1.2.3", []string{"1.2.3", "v1.2.3"}, []string{"1.2.4", "1.2.3-rc.1"}},
		{"=1.2", []string{"1.2.0", "1.2.7"}, []string{"1.3.0", "1.1.9"}},
		{"==v1.2.3", []string{"1.2.3"}, []string{"1.2.4"}},
		{"v1.2.3", []string{"1.2.3"}, []string{"1.2.4"}},
		{"^1.2.3", []string{"1.2.3", "1.9.0", "1.99.99"}, []string{"1.2.2", "2.0.0", "2.0.0-rc.1", "1.2.3-rc.1"}},
		{"^0.2.3", []string{"0.2.3", "0.2.9"}, []string{"0.3.0", "0.2.2", "1.0.0"}},
		{"^0.0.3", []string{"0.0.3"}, []string{"0.0.4", "0.0.2"}},
		{"^1", []string{"1.0.0", "1.5.0"}, []string{"2.0.0", "0.9.0"}},
		{"^1.0.0", []string{"1.0.0", "1.1.0"}, []string{"2.0.0", "0.9.0"}},
		{"~1.2.3", []string{"1.2.3", "1.2.9"}, []string{"1.3.0", "1.2.2", "2.0.0"}},
		{"~1.1.0", []string{"1.1.0", "1.1.7"}, []string{"1.2.0", "1.0.9"}},
		{"~1", []string{"1.0.0", "1.9.0"}, []string{"2.0.0"}},
		{"1.1.x", []string{"1.1.0", "1.1.9"}, []string{"1.2.0", "1.0.9"}},
		{"1.x", []string{"1.0.0", "1.9.9"}, []string{"2.0.0", "0.9.0"}},
		{"1.*", []string{"1.0.0"}, []string{"2.0.0"}},
		{"1.2.3 - 2.3.4", []string{"1.2.3", "2.0.0", "2.3.4"}, []string{"1.2.2", "2.3.5"}},
		{"1.2 - 2.3", []string{"1.2.0", "2.3.9"}, []string{"1.1.9", "2.4.0"}},
		{">=1.2.0", []string{"1.2.0", "2.0.0"}, []string{"1.1.9", "1.2.0-rc.1"}},
		{">1.2.0", []string{"1.2.1"}, []string{"1.2.0"}},
		{"<2.0.0", []string{"1.9.9", "2.0.0-rc.1"}, []string{"2.0.0", "2.0.1"}},
		{"<=2.0.0", []string{"2.0.0", "1.0.0"}, []string{"2.0.1"}},
		{">=1.0.0 <2.0.0", []string{"1.0.0", "1.9.9"}, []string{"0.9.0", "2.0.0"}},
		{">=1.2.0, <2.0.0", []string{"1.2.0", "1.9.9"}, []string{"1.1.0", "2.0.0"}},
		{"1.0.0 || 2.0.0", []string{"1.0.0", "2.0.0"}, []string{"1.5.0", "3.0.0"}},
		{"^1.0.0 || ^3.0.0", []string{"1.4.0", "3.1.0"}, []string{"2.0.0"}},
		{">=1.0.0-rc.1", []string{"1.0.0-rc.1", "1.0.0-rc.2", "1.0.0"}, []string{"1.0.0-rc.0", "1.0.0-beta"}},
		{"1.0.0-rc.1", []string{"1.0.0-rc.1"}, []string{"1.0.0"}},
	}
	for _, tc := range cases {
		r, err := parseVersionRange(tc.constraint)
		if err != nil {
			t.Fatalf("parseVersionRange(%q): %v", tc.constraint, err)
		}
		for _, v := range tc.yes {
			if !r.matches(v) {
				t.Errorf("%q should match %q", tc.constraint, v)
			}
		}
		for _, v := range tc.no {
			if r.matches(v) {
				t.Errorf("%q should not match %q", tc.constraint, v)
			}
		}
	}
}

func TestParseVersionRangeRefuses(t *testing.T) {
	for _, bad := range []string{"=", "^", ">=", "1.2.3.4", "abc", "~ ", ">=1.0.0,<", "1.0.0 ||", "1.x.3", ">*", "latest", "1.0.0 - ", "1.0.0-"} {
		if _, err := parseVersionRange(bad); !errors.Is(err, errBadConstraint) {
			t.Errorf("parseVersionRange(%q) = %v, want errBadConstraint", bad, err)
		}
	}
}
