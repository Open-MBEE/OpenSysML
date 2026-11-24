package export

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// A body parameter typed by a conjugate (`in a : ~B`) owns a ConjugatedPortTyping, as a usage
// typed so does, and the bound of a typing list (`in b : A, B[1]`) is the parameter's.
func TestBodyParameterTypingListReadsBackFromTheGraphAlone(t *testing.T) {
	alone := withoutSourceLayout(graphOfNotation(t, `package P {
    port def A;
    port def B;
    calc def F {
        in xs : A[*];
        xs->collect { in a : ~B; a }
        xs->collect { in b : A, B[1]; b }
        xs->collect { in c defined by ~A, ~B; c }
    }
}
`))
	conjugated := 0
	for _, triple := range alone.Triples() {
		if triple.Predicate.Value == rdf.RDFType && triple.Object.Value == rdf.SysML+mConjugatedPortTyping {
			conjugated++
		}
	}
	if conjugated != 3 {
		t.Errorf("the graph states %d conjugated port typings, want 3", conjugated)
	}
	notation, err := ToSysML(alone)
	if err != nil {
		t.Fatalf("ToSysML: %v", err)
	}
	for _, head := range []string{"in a : ~B;", "in b : A, B[1];", "in c : ~A, ~B;"} {
		if !strings.Contains(string(notation), head) {
			t.Errorf("%q does not come back:\n%s", head, notation)
		}
	}
}
