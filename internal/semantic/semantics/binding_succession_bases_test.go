package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// TestBindingAndSuccessionSubsetTheirKerMLBases: a binding subsets Links::selfLinks, a succession
// Occurrences::happensBeforeLinks (KerML 1.1 §8.4.4.7.1, §8.4.4.8.1); ends named by what they redefine.
func TestBindingAndSuccessionSubsetTheirKerMLBases(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
    part def D {
        part a;
        part b;
        binding ab bind a = b;
        action x;
        action y;
        succession s first x then y;
    }
}
`)
	d := nested(t, sym(t, root, "P").Scope, "D")
	ab := nested(t, d.Scope, "ab")
	s := nested(t, d.Scope, "s")

	if !m.conformsByName(ab, "Links::SelfLink") {
		t.Errorf("binding ab does not conform to Links::SelfLink; supertypes %v", names(m.DirectSupertypes(ab)))
	}
	if !m.conformsByName(s, "Occurrences::HappensBefore") {
		t.Errorf("succession s does not conform to Occurrences::HappensBefore; supertypes %v", names(m.DirectSupertypes(s)))
	}
	var endNames []string
	for _, end := range m.ConnectorObjectEnds(ab) {
		endNames = append(endNames, end.Name)
	}
	if want := []string{"thisThing", "sameThing"}; !equalStrings(endNames, want) {
		t.Errorf("binding end names = %v, want %v", endNames, want)
	}
	for _, usage := range []*symbols.Symbol{ab, s} {
		if m.UsageMayTimeVary(usage) {
			t.Errorf("%s varies in time, but a SelfLink or HappensLink does not (SysML v2 §8.3.6.4)", usage.Name)
		}
	}
}

func names(syms []*symbols.Symbol) []string {
	out := make([]string, 0, len(syms))
	for _, s := range syms {
		out = append(out, s.Name)
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
