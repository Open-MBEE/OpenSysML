package export_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
)

// specializationEdges reads the specialization edges a graph describes, in
// either of the two shapes the notation writes: the clause of a declaration
// (`classifier A :> B`) states the general on the specific type itself, and the
// keyword-first member (`subtype A specializes B`) states both ends on the
// Specialization element it declares.
func specializationEdges(t *testing.T, g *rdf.Graph) []string {
	t.Helper()
	var edges []string
	for _, s := range g.Subjects() {
		if !s.IsIRI() {
			continue
		}
		for _, general := range g.Objects(s, rdf.SysML+"specializes") {
			edges = append(edges, edge(t, g, s, general))
		}
		if rdf.LocalName(g.Type(s)) != "Specialization" {
			continue
		}
		specifics := g.Objects(s, rdf.SysML+"specific")
		generals := g.Objects(s, rdf.SysML+"general")
		if len(specifics) != 1 || len(generals) != 1 {
			t.Fatalf("<%s> relates %d specific and %d general ends, want one of each",
				s, len(specifics), len(generals))
		}
		edges = append(edges, edge(t, g, specifics[0], generals[0]))
	}
	sort.Strings(edges)
	return edges
}

// edge names one specialization edge by the two elements it relates, so the two
// notations compare independently of which element carries the edge.
func edge(t *testing.T, g *rdf.Graph, specific, general rdf.Term) string {
	t.Helper()
	return elementName(t, g, specific) + " :> " + elementName(t, g, general)
}

// elementName is an element's qualified name, or the text of a reference to one
// this document does not declare.
func elementName(t *testing.T, g *rdf.Graph, term rdf.Term) string {
	t.Helper()
	if term.IsIRI() {
		if name, ok := g.Lexical(term, rdf.SysML+"qualifiedName"); ok {
			return name
		}
	}
	return strings.Trim(term.String(), `"`)
}

// kermlTurtleOf converts KerML notation to Turtle and parses the result. The
// keyword-first relationship members are KerML-only forms.
func kermlTurtleOf(t *testing.T, name, src string) *rdf.Graph {
	t.Helper()
	data, err := export.Convert(name, []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph, err := rdf.ParseTurtle(data)
	if err != nil {
		t.Fatalf("parse turtle: %v\n%s", err, data)
	}
	return graph
}

// TestKeywordFirstSpecializationDescribesTheSameGraph is the end-ordering
// contract of F86–F91: a keyword-first specialization relates A to B in that
// order, so it describes the same edge the `:>` clause does. Before the
// relationship element existed, the two ends were two same-kind relationships of
// an anonymous usage and the graph said the relationship specialized both.
func TestKeywordFirstSpecializationDescribesTheSameGraph(t *testing.T) {
	keywordFirst := kermlTurtleOf(t, "keyword-first.kerml", `package P {
    classifier A;
    classifier B;
    specialization Gen subtype A specializes B;
}`)
	inline := kermlTurtleOf(t, "inline.kerml", `package P {
    classifier A :> B;
    classifier B;
}`)
	want := specializationEdges(t, inline)
	got := specializationEdges(t, keywordFirst)
	if len(want) != 1 || want[0] != "P::A :> P::B" {
		t.Fatalf("the `:>` form describes %v, want one P::A :> P::B edge", want)
	}
	if len(got) != len(want) || got[0] != want[0] {
		t.Errorf("keyword-first form describes %v, want %v", got, want)
	}
}

// TestKeywordFirstRelationshipEndsAreOrdered pins the ordered ends of each
// keyword-first form, which is what makes source and target queryable.
func TestKeywordFirstRelationshipEndsAreOrdered(t *testing.T) {
	g := kermlTurtleOf(t, "ends.kerml", `package P {
    classifier A;
    classifier B;
    feature f : A;
    feature g : B;
    specialization Gen subtype A specializes B;
    specialization Sub subset f subsets g;
    inverting i inverse f of g;
    disjoining D disjoint A from B;
}`)
	cases := []struct {
		element                string
		metaclass              string
		source, target         string
		sourceName, targetName string
	}{
		{"urn:sysmlv2:element:P__Gen", "Specialization", "specific", "general", "P::A", "P::B"},
		{"urn:sysmlv2:element:P__Sub", "Subsetting", "subsettingFeature", "subsettedFeature", "P::f", "P::g"},
		{"urn:sysmlv2:element:P__i", "FeatureInverting", "invertingFeature", "featureInverted", "P::f", "P::g"},
		{"urn:sysmlv2:element:P__D", "Disjoining", "typeDisjoined", "disjoiningType", "P::A", "P::B"},
	}
	for _, c := range cases {
		wantType(t, g, c.element, c.metaclass)
		for _, end := range []struct{ property, name string }{{c.source, c.sourceName}, {c.target, c.targetName}} {
			objects := g.Objects(rdf.IRI(c.element), rdf.SysML+end.property)
			if len(objects) != 1 {
				t.Errorf("<%s> states %d %s ends, want 1", c.element, len(objects), end.property)
				continue
			}
			if got := elementName(t, g, objects[0]); got != end.name {
				t.Errorf("<%s> %s = %q, want %q", c.element, end.property, got, end.name)
			}
		}
	}
}

// A relationship member states its own visibility, so the graph must carry the
// keyword the wrapping membership used to hold.
func TestKeywordFirstRelationshipKeepsItsVisibility(t *testing.T) {
	const src = `package P {
    classifier A;
    classifier B;
    private specialization Gen subtype A specializes B;
}
`
	g := kermlTurtleOf(t, "visibility.kerml", src)
	vis, ok := g.Lexical(rdf.IRI("urn:sysmlv2:element:P__Gen"), rdf.SysML+"visibility")
	if !ok || vis != "private" {
		t.Errorf("visibility = %q (present=%t), want private", vis, ok)
	}
	turtle, err := export.Convert("visibility.kerml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	back, err := export.Convert("visibility.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if !strings.Contains(string(back), "private specialization Gen") {
		t.Errorf("round trip lost the declared visibility:\n%s", back)
	}
}

// TestKeywordFirstRelationshipRoundTrips checks the graph is enough to write the
// notation back, ends included.
func TestKeywordFirstRelationshipRoundTrips(t *testing.T) {
	const src = `package P {
    classifier A;
    classifier B;
    specialization Gen subtype A specializes B;
}
`
	turtle, err := export.Convert("round.kerml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	back, err := export.Convert("round.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if !strings.Contains(string(back), "specialization Gen subtype A specializes B") {
		t.Errorf("round trip lost the relationship member:\n%s", back)
	}
}

// disjoiningSrc writes the two shapes a keyword-first Disjoining takes: the
// Kernel Semantic Library's feature-chain ends, and a named one with a body.
const disjoiningSrc = `package P {
    class Occ {
        feature successors : Occ;
        feature predecessors : Occ;
    }
    feature earlierOccurrence : Occ;
    feature laterOccurrence : Occ;
    disjoint earlierOccurrence.successors from laterOccurrence.predecessors;
    classifier A;
    classifier B;
    private disjoining D disjoint A from B {
        doc /* A and B share no instance. */
    }
}
`

// TestDisjoiningRoundTripsFromTheGraphAlone requires the ordered ends of a
// Disjoining, not the source text, to carry `disjoint X from Y` back to notation.
func TestDisjoiningRoundTripsFromTheGraphAlone(t *testing.T) {
	first, err := export.Convert("disjoining.kerml", []byte(disjoiningSrc), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	back := string(structuralRoundTrip(t, "disjoining.kerml", first))
	for _, want := range []string{
		"disjoint earlierOccurrence.successors from laterOccurrence.predecessors;",
		"private disjoining D disjoint A from B {",
		"doc /* A and B share no instance. */",
	} {
		if !strings.Contains(back, want) {
			t.Errorf("the graph alone lost %q:\n%s", want, back)
		}
	}
	if strings.Contains(back, "disjoint from") {
		t.Errorf("the graph alone wrote a Disjoining as a declaration clause:\n%s", back)
	}
}

// TestDisjoiningOrientationIsLoadBearing swaps the two end properties in the
// graph and expects the notation to swap its ends: which type is disjoined from
// which is stated by the graph, not recovered from the text.
func TestDisjoiningOrientationIsLoadBearing(t *testing.T) {
	turtle, err := export.Convert("disjoining.kerml", []byte(disjoiningSrc), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	structural := string(withoutTriples(t, turtle, "sysx:sourceText"))
	swapped := strings.NewReplacer(
		"sysml:typeDisjoined", "sysml:SWAP",
		"sysml:disjoiningType", "sysml:typeDisjoined",
	).Replace(structural)
	swapped = strings.ReplaceAll(swapped, "sysml:SWAP", "sysml:disjoiningType")
	if swapped == structural {
		t.Fatal("the graph states no Disjoining ends to swap")
	}
	back := toNotation(t, []byte(swapped))
	for _, want := range []string{
		"disjoint laterOccurrence.predecessors from earlierOccurrence.successors;",
		"private disjoining D disjoint B from A {",
	} {
		if !strings.Contains(back, want) {
			t.Errorf("swapping the graph's ends did not swap the notation's, want %q:\n%s", want, back)
		}
	}
}

// The declaration clause `disjoint from C` is a property of the declared type,
// not a Disjoining element, and stays so.
func TestDisjoiningClauseStaysOnItsDeclaration(t *testing.T) {
	const src = `package P {
    classifier A;
    classifier B;
    classifier C specializes A disjoint from B;
}
`
	first, err := export.Convert("clause.kerml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	g, err := rdf.ParseTurtle(first)
	if err != nil {
		t.Fatalf("parse turtle: %v\n%s", err, first)
	}
	for _, s := range g.Subjects() {
		if rdf.LocalName(g.Type(s)) == "Disjoining" {
			t.Errorf("the clause form declared a Disjoining element <%s>", s)
		}
	}
	objects := g.Objects(rdf.IRI("urn:sysmlv2:element:P__C"), rdf.SysML+"disjointFrom")
	if len(objects) != 1 || elementName(t, g, objects[0]) != "P::B" {
		t.Errorf("P::C disjointFrom = %v, want P::B alone", objects)
	}
	back := string(structuralRoundTrip(t, "clause.kerml", first))
	if !strings.Contains(back, " C specializes A disjoint from B;") {
		t.Errorf("the graph alone lost the declaration clause:\n%s", back)
	}
}
