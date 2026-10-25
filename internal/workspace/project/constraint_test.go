package project

import (
	"errors"
	"testing"
)

func TestConstraintMatches(t *testing.T) {
	cases := []struct {
		constraint string
		yes, no    []string
	}{
		{"", []string{"0.0.1", "1.0.0", "v9.9.9", "1.0.0-rc.1"}, []string{"", "x"}},
		{"1.2.3", []string{"1.2.3", "v1.2.3"}, []string{"1.2.4", "1.2.3-rc.1"}},
		{"=1.2", []string{"1.2.0"}, []string{"1.2.1"}},
		{"==v1.2.3", []string{"1.2.3"}, []string{"1.2.4"}},
		{"^1.2.3", []string{"1.2.3", "1.9.0", "1.99.99"}, []string{"1.2.2", "2.0.0", "2.0.0-rc.1", "1.2.3-rc.1"}},
		{"^0.2.3", []string{"0.2.3", "0.2.9"}, []string{"0.3.0", "0.2.2", "1.0.0"}},
		{"^0.0.3", []string{"0.0.3"}, []string{"0.0.4", "0.0.2"}},
		{"^1", []string{"1.0.0", "1.5.0"}, []string{"2.0.0", "0.9.0"}},
		{"~1.2.3", []string{"1.2.3", "1.2.9"}, []string{"1.3.0", "1.2.2", "2.0.0"}},
		{"~1.2", []string{"1.2.0", "1.2.7"}, []string{"1.3.0"}},
		{">=1.2.0", []string{"1.2.0", "2.0.0"}, []string{"1.1.9", "1.2.0-rc.1"}},
		{">1.2.0", []string{"1.2.1"}, []string{"1.2.0"}},
		{"<2.0.0", []string{"1.9.9", "2.0.0-rc.1"}, []string{"2.0.0", "2.0.1"}},
		{"<=2.0.0", []string{"2.0.0", "1.0.0"}, []string{"2.0.1"}},
		{">=1.2.0, <2.0.0", []string{"1.2.0", "1.9.9"}, []string{"1.1.0", "2.0.0"}},
		{">=1.2.0 <1.5.0", []string{"1.4.9"}, []string{"1.5.0"}},
	}
	for _, tc := range cases {
		c, err := ParseConstraint(tc.constraint)
		if err != nil {
			t.Fatalf("ParseConstraint(%q): %v", tc.constraint, err)
		}
		for _, v := range tc.yes {
			if !c.Matches(v) {
				t.Errorf("%q should match %q", tc.constraint, v)
			}
		}
		for _, v := range tc.no {
			if c.Matches(v) {
				t.Errorf("%q should not match %q", tc.constraint, v)
			}
		}
	}
}

func TestParseConstraintRefuses(t *testing.T) {
	for _, bad := range []string{"=", "^", ">=", "1.2.3.4", "1.x", "abc", "~ ", ">=1.0.0,<", "1.0.0 || 2.0.0"} {
		if _, err := ParseConstraint(bad); !errors.Is(err, ErrBadConstraint) {
			t.Errorf("ParseConstraint(%q) = %v, want ErrBadConstraint", bad, err)
		}
	}
}

func TestConstraintExactAndLatest(t *testing.T) {
	for constraint, want := range map[string]string{"1.2.3": "1.2.3", "=v1.2": "1.2.0", "^1.2.3": "", "": "", ">=1.0.0, <2.0.0": ""} {
		c, err := ParseConstraint(constraint)
		if err != nil {
			t.Fatal(err)
		}
		if got := c.Exact(); got != want {
			t.Errorf("Exact(%q) = %q, want %q", constraint, got, want)
		}
	}
	versions := []string{"1.0.0", "v1.2.0", "1.2.0", "1.10.0", "2.0.0-rc.1", "2.0.0", "0.9.0"}
	for constraint, want := range map[string]string{"": "2.0.0", "^1.0.0": "1.10.0", "~1.2.0": "1.2.0", "<2.0.0": "2.0.0-rc.1", ">=3.0.0": "", "1.2": "1.2.0"} {
		c, _ := ParseConstraint(constraint)
		if got := c.Latest(versions); got != want {
			t.Errorf("Latest(%q) = %q, want %q", constraint, got, want)
		}
	}
}

func TestSatisfiesRanges(t *testing.T) {
	p := &Interchange{Version: "1.4.2"}
	for constraint, want := range map[string]bool{"1.4.2": true, "^1.2.0": true, "~1.4.0": true, ">=1.0.0, <2.0.0": true, "~1.3.0": false, "2.0.0": false, "": false, "garbage": false} {
		if got := p.satisfies(constraint); got != want {
			t.Errorf("satisfies(%q) = %v, want %v", constraint, got, want)
		}
	}
}
