package export_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
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

// A graph from another tool may link only some of a message's ends from the
// flow itself: the `to b` membership is a sysml:ownedMembership of the flow
// while the `from a` membership only names the flow back. The ends still come
// back in written order, read from their memberIndexes or from the flow's
// own source and target.

// partlyLinkedMessage strips the layout from messageEndsModel's graph and
// unlinks the first end's membership (pend0_om, the `from a` end) from the
// flow: the flow keeps its links to pend1 only. It returns the stripped graph.
func partlyLinkedMessage(t *testing.T) *rdf.Graph {
	t.Helper()
	turtle, err := convert.Convert("m.sysml", []byte(messageEndsModel), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph, err := rdf.ParseTurtle(withoutLayout(t, turtle))
	if err != nil {
		t.Fatalf("parse turtle: %v", err)
	}
	flow := rdf.IRI(rdf.Element + "M__Talk__m")
	first := map[string]bool{rdf.Expression + "M__Talk__m_pend0_om": true, rdf.Expression + "M__Talk__m_pend0": true}
	json := map[string]bool{}
	for _, property := range []string{"ownedMembership", "ownedRelationship", "ownedFeatureMembership", "ownedFeature", "parameter"} {
		json[rdf.AnnotationJSON+property] = true
	}
	stripped := rdf.NewGraph()
	stripped.Prefixes = graph.Prefixes
	for _, triple := range graph.Triples() {
		if triple.Subject == flow && (first[triple.Object.Value] || json[triple.Predicate.Value]) {
			continue
		}
		stripped.AddTriple(triple)
	}
	for _, want := range []string{
		"expr:M__Talk__m_pend0_om",
		"sysml:membershipOwningNamespace elmt:M__Talk__m",
	} {
		if !strings.Contains(string(rdf.WriteTurtle(stripped)), want) {
			t.Fatalf("the first end's membership should still state %q:\n%s", want, rdf.WriteTurtle(stripped))
		}
	}
	return stripped
}

func TestMessageEndsComeBackOrderedWhenPartlyLinked(t *testing.T) {
	back, err := convert.Convert("m.ttl", rdf.WriteTurtle(partlyLinkedMessage(t)), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if !strings.Contains(string(back), "message m from a to b;\n") {
		t.Errorf("the partly linked message lost its end order:\n%s", back)
	}
}

// withoutFlowTriples drops the flow's own triples on each named property,
// plus the JSON annotation collections carrying the same keys.
func withoutFlowTriples(graph *rdf.Graph, flow rdf.Term, properties ...string) *rdf.Graph {
	dropped := map[string]bool{}
	for _, property := range properties {
		dropped[rdf.SysML+property] = true
		dropped[rdf.AnnotationJSON+property] = true
	}
	out := rdf.NewGraph()
	out.Prefixes = graph.Prefixes
	for _, triple := range graph.Triples() {
		if triple.Subject == flow && dropped[triple.Predicate.Value] {
			continue
		}
		out.AddTriple(triple)
	}
	return out
}

func messageFlow(g *rdf.Graph) rdf.Term { return rdf.IRI(rdf.Element + "M__Talk__m") }

func TestPartlyLinkedMessageEndsWithoutAnOrderAreRefused(t *testing.T) {
	graph := partlyLinkedMessage(t)
	stripped := withoutFlowTriples(graph, messageFlow(graph), "sourceFeature", "targetFeature", "relatedFeature")
	_, err := convert.Convert("m.ttl", rdf.WriteTurtle(stripped), convert.FormatTurtle, convert.FormatSysML)
	var unsupported *export.UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("expected an UnsupportedError, got %v", err)
	}
	if !strings.Contains(err.Error(), "memberIndex") {
		t.Errorf("the refusal should name the missing order:\n%v", err)
	}
}

func TestPartlyLinkedMessageEndsOrderByMemberIndex(t *testing.T) {
	graph := partlyLinkedMessage(t)
	stripped := withoutFlowTriples(graph, messageFlow(graph), "sourceFeature", "targetFeature", "relatedFeature")
	stripped.AddTriple(rdf.Triple{
		Subject:   rdf.IRI(rdf.Expression + "M__Talk__m_pend0_om"),
		Predicate: rdf.OpenSysMLTerm("memberIndex"),
		Object:    rdf.Int(0),
	})
	stripped.AddTriple(rdf.Triple{
		Subject:   rdf.IRI(rdf.Expression + "M__Talk__m_pend1_om"),
		Predicate: rdf.OpenSysMLTerm("memberIndex"),
		Object:    rdf.Int(1),
	})
	back, err := convert.Convert("m.ttl", rdf.WriteTurtle(stripped), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if !strings.Contains(string(back), "message m from a to b;\n") {
		t.Errorf("the memberIndexes did not order the ends:\n%s", back)
	}
}

func TestPartlyLinkedMessageEndsRefuseADisagreeingOrder(t *testing.T) {
	graph := partlyLinkedMessage(t)
	stripped := rdf.NewGraph()
	stripped.Prefixes = graph.Prefixes
	for _, triple := range graph.Triples() {
		stripped.AddTriple(triple)
	}
	stripped.AddTriple(rdf.Triple{
		Subject:   rdf.IRI(rdf.Expression + "M__Talk__m_pend0_om"),
		Predicate: rdf.OpenSysMLTerm("memberIndex"),
		Object:    rdf.Int(1),
	})
	stripped.AddTriple(rdf.Triple{
		Subject:   rdf.IRI(rdf.Expression + "M__Talk__m_pend1_om"),
		Predicate: rdf.OpenSysMLTerm("memberIndex"),
		Object:    rdf.Int(0),
	})
	_, err := convert.Convert("m.ttl", rdf.WriteTurtle(stripped), convert.FormatTurtle, convert.FormatSysML)
	var unsupported *export.UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("expected an UnsupportedError, got %v", err)
	}
	if !strings.Contains(err.Error(), "disagree") {
		t.Errorf("the refusal should name the disagreement:\n%v", err)
	}
}
