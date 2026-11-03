package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestKerMLBindingAndSuccessionEndsAreNamedByTheirBases: a KerML binding's ends are SelfLink's
// thisThing/sameThing and a succession's HappensBefore's earlierOccurrence/laterOccurrence.
func TestKerMLBindingAndSuccessionEndsAreNamedByTheirBases(t *testing.T) {
	m, root := buildModelWithStdlibNamedKind(t, "links.kerml", source.KindKerML, `package P {
    class A;
    class Q {
        feature a : A;
        feature b : A;
        binding ab of a = b;
        step x;
        step y;
        succession s first x then y;
    }
}
`)
	q := nested(t, sym(t, root, "P").Scope, "Q")
	for _, c := range []struct {
		name string
		want []string
	}{{"ab", []string{"thisThing", "sameThing"}}, {"s", []string{"earlierOccurrence", "laterOccurrence"}}} {
		usage := nested(t, q.Scope, c.name)
		var got []string
		if c.name == "s" {
			// A KerML succession is no connector object usage; its declared ends are named as the writer names them.
			got = []string{m.DeclaredSuccessionEndName(usage, 0), m.DeclaredSuccessionEndName(usage, 1)}
		}
		for _, end := range m.ConnectorObjectEnds(usage) {
			got = append(got, end.Name)
		}
		if !equalStrings(got, c.want) {
			t.Errorf("%s end names = %v, want %v; supertypes %v", c.name, got, c.want, names(m.DirectSupertypes(usage)))
		}
	}
}
