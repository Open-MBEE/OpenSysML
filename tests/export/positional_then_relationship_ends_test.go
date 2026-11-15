package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// A `then x;` reaching a nameless member by position — a bare `entry;`, an unnamed action — states it as
// its sourceFeature and Relationship::source beside the positional binding, as the pilot derives them.
func TestPositionalThenSuccessionStatesItsRelationshipEnds(t *testing.T) {
	const model = `package P {
    action def Step;
    state def T {
        entry; then idle;
        state idle;
    }
    action def A {
        action : Step;
        then action b : Step;
    }
}
`
	turtle, err := convert.Convert("p.sysml", []byte(model), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatalf("parse turtle: %v", err)
	}
	for _, c := range []struct{ succession, source, target string }{
		{"P__T___401", "P__T___400", "P__T__idle"},
		{"P__A___402", "P__A___400", "P__A__b"},
	} {
		subject := rdf.IRI(rdf.Element + c.succession)
		if got, ok := graph.Object(subject, rdf.OpenSysML+"sourceMember"); !ok || got.Value != rdf.Element+c.source {
			t.Fatalf("%s should bind its source by position to %s, got %v (%t)", c.succession, c.source, got, ok)
		}
		for _, pair := range [][2]string{{"sourceFeature", c.source}, {"source", c.source}, {"targetFeature", c.target}, {"target", c.target}} {
			got, ok := graph.Object(subject, rdf.SysML+pair[0])
			if !ok || got.Value != rdf.Element+pair[1] {
				t.Errorf("%s %s = %v (%t), want %s", c.succession, pair[0], got, ok, pair[1])
			}
		}
	}
	structuralRoundTrip(t, "p", turtle)
	back, err := convert.Convert("p.ttl", withoutTriples(t, turtle, "sysx:sourceText"), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("graph-only hop: %v", err)
	}
	for _, want := range []string{"entry;\n        then idle;", "action : Step;\n        then action b : Step;"} {
		if !strings.Contains(string(back), want) {
			t.Errorf("the graph-only hop should write %q back:\n%s", want, back)
		}
	}
}
