package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// A graph from another tool may state a message's ends only from the
// memberships' side: each ParameterMembership names the flow in
// sysml:membershipOwningNamespace (or sysml:owningRelatedElement) and its end
// in sysml:ownedMemberParameter, with no sysml:ownedMembership,
// sysml:ownedRelationship or sysml:parameter under the flow itself. The reader
// accepts such memberships, so the message must still find its `from` and `to`
// through them, in the order the graph states them.
func TestMessageEndsComeBackFromStandaloneMemberships(t *testing.T) {
	const model = `package M {
    part a;
    part b;
    occurrence def Talk {
        message m from a to b;
    }
}
`
	turtle, err := convert.Convert("m.sysml", []byte(model), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph, err := rdf.ParseTurtle(withoutLayout(t, turtle))
	if err != nil {
		t.Fatalf("parse turtle: %v", err)
	}
	flow := rdf.IRI(rdf.Element + "M__Talk__m")
	dropped := map[string]bool{}
	for _, property := range []string{"ownedMembership", "ownedRelationship", "ownedFeatureMembership", "ownedFeature", "parameter"} {
		dropped[rdf.SysML+property] = true
		dropped[rdf.AnnotationJSON+property] = true
	}
	stripped := rdf.NewGraph()
	stripped.Prefixes = graph.Prefixes
	removed := 0
	for _, triple := range graph.Triples() {
		if triple.Subject == flow && dropped[triple.Predicate.Value] {
			removed++
			continue
		}
		stripped.AddTriple(triple)
	}
	if removed == 0 {
		t.Fatalf("the flow states no ownership links to drop:\n%s", turtle)
	}
	for _, want := range []string{
		"sysml:membershipOwningNamespace elmt:M__Talk__m",
		"sysml:ownedMemberParameter expr:M__Talk__m_pend0",
	} {
		if !strings.Contains(string(rdf.WriteTurtle(stripped)), want) {
			t.Fatalf("the memberships should still state %q:\n%s", want, rdf.WriteTurtle(stripped))
		}
	}
	back, err := convert.Convert("m.ttl", rdf.WriteTurtle(stripped), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation from the memberships alone: %v", err)
	}
	if !strings.Contains(string(back), "message m from a to b;\n") {
		t.Errorf("the message lost its ends:\n%s", back)
	}
	if strings.Contains(string(back), "event occurrence") {
		t.Errorf("a message end was written back as a member:\n%s", back)
	}
}
