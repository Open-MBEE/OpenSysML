package rdf

import (
	"errors"
	"strings"
	"testing"
)

// The Turtle writer declares the annotation namespace under the service's own
// prefix, so an annotated graph reads back with the prefix Flexo uses.
func TestAnnotationPrefixIsDeclared(t *testing.T) {
	g := NewGraph()
	g.Add(IRI(Element+"P"), AnnotationJSONTerm("ownedMember"), String(`[{"@id":"P__A"},{"@id":"P__B"}]`))
	turtle := string(WriteTurtle(g))
	if !strings.Contains(turtle, "@prefix json: <urn:sysmlv2:annotation:json:> .") {
		t.Errorf("the json prefix is not declared:\n%s", turtle)
	}
	if !strings.Contains(turtle, `json:ownedMember "[{\"@id\":\"P__A\"},{\"@id\":\"P__B\"}]"`) {
		t.Errorf("the annotation is not written under the prefix:\n%s", turtle)
	}
}

// A subject stating a sysml: property more than once gets one annotation holding
// the members as JSON in triple order; single values and other namespaces do not.
func TestAnnotateCollections(t *testing.T) {
	g := NewGraph()
	p := IRI(Element + "P")
	g.Add(p, IRI(RDFType), SysMLTerm("Package"))
	g.Add(p, SysMLTerm("declaredName"), String("P"))
	g.Add(p, SysMLTerm("ownedMember"), IRI(Element+"P__B"))
	g.Add(p, SysMLTerm("ownedMember"), IRI(Element+"P__A"))
	g.Add(p, SysMLTerm("ownedMember"), IRI(Expression+"P__A_pa0"))
	g.Add(p, SysMLTerm("aliasIds"), String("x"))
	g.Add(p, SysMLTerm("aliasIds"), String("y"))
	g.Add(p, OpenSysMLTerm("memberIndex"), Int(0))
	g.Add(p, OpenSysMLTerm("memberIndex"), Int(1))
	if err := AnnotateCollections(g); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"ownedMember": `[{"@id":"P__B"},{"@id":"P__A"},{"@id":"P__A_pa0"}]`,
		"aliasIds":    `["x","y"]`,
	}
	for key, text := range want {
		objects := g.Objects(p, AnnotationJSON+key)
		if len(objects) != 1 || objects[0] != String(text) {
			t.Errorf("json:%s = %v, want %q", key, objects, text)
		}
	}
	for _, key := range []string{"declaredName", "memberIndex"} {
		if g.HasProperty(p, AnnotationJSON+key) {
			t.Errorf("json:%s was written for a property that is not a sysml: collection", key)
		}
	}
	if got := len(g.Objects(p, SysML+"ownedMember")); got != 3 {
		t.Errorf("the typed triples were not kept: %d ownedMember objects", got)
	}
	// Annotating again changes nothing.
	before := g.Len()
	if err := AnnotateCollections(g); err != nil {
		t.Fatal(err)
	}
	if g.Len() != before {
		t.Errorf("annotating twice added %d triples", g.Len()-before)
	}
}

// An id both an element and an expression node carry resolves, from an annotation
// alone, in the referrer's namespace; a referrer in neither has it refused.
func TestMaterializedReferencesFollowTheReferrersNamespace(t *testing.T) {
	element, node := IRI(Element+"P__x_pvalue_pa0"), IRI(Expression+"P__x_pvalue_pa0")
	base := func() *Graph {
		g := NewGraph()
		g.Add(element, IRI(RDFType), SysMLTerm("PartDefinition"))
		g.Add(node, IRI(RDFType), SysMLTerm("LiteralInteger"))
		return g
	}
	for _, tc := range []struct {
		referrer, want Term
	}{
		{IRI(Element + "P"), element},
		{IRI(Expression + "P__x_pvalue"), node},
	} {
		g := base()
		g.Add(tc.referrer, AnnotationJSONTerm("argument"), String(`[{"@id":"P__x_pvalue_pa0"}]`))
		out, err := ReconcileCollections(g)
		if err != nil {
			t.Fatalf("%s: %v", tc.referrer, err)
		}
		if got := out.Objects(tc.referrer, SysML+"argument"); len(got) != 1 || got[0] != tc.want {
			t.Errorf("%s resolved the member to %v, want %s", tc.referrer, got, tc.want)
		}
	}
	g := base()
	foreign := IRI("urn:other:P")
	g.Add(foreign, AnnotationJSONTerm("argument"), String(`[{"@id":"P__x_pvalue_pa0"}]`))
	_, err := ReconcileCollections(g)
	var refused *AnnotationError
	if !errors.As(err, &refused) || refused.Subject != foreign.Value || refused.Key != "argument" {
		t.Fatalf("an id no namespace tells apart was not refused: %v", err)
	}
	for _, want := range []string{element.String(), node.String(), "cannot tell which"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal lacks %q: %v", want, err)
		}
	}
}

// A subject outside the element and expression namespaces is no reference
// target, however its local name reads: the @id stays an element IRI and dangles.
func TestForeignSubjectsAreNotReferenceTargets(t *testing.T) {
	subject, foreign := IRI(Element+"P"), IRI("urn:other:A")
	g := NewGraph()
	g.Add(subject, IRI(RDFType), SysMLTerm("Package"))
	g.Add(foreign, IRI(RDFType), SysMLTerm("PartDefinition"))
	g.Add(subject, AnnotationJSONTerm("ownedMember"), String(`[{"@id":"A"}]`))
	out, err := ReconcileCollections(g)
	if err != nil {
		t.Fatal(err)
	}
	if got := out.Objects(subject, SysML+"ownedMember"); len(got) != 1 || got[0] != IRI(Element+"A") {
		t.Errorf("the member resolved to %v, want the dangling %s", got, IRI(Element+"A"))
	}
}

// A graph holds each triple once, so an annotation that repeats a member cannot
// round-trip; it is refused, whether the member is a reference or a primitive.
func TestRepeatedAnnotationMembersAreRefused(t *testing.T) {
	subject, wheel := IRI(Element+"P"), IRI(Element+"P__Wheel")
	for key, literal := range map[string]string{
		"ownedMember": `[{"@id":"P__Wheel"},{"@id":"P__Wheel"}]`,
		"aliasIds":    `["a","b","a"]`,
		"value":       `[1,2,1]`,
	} {
		g := NewGraph()
		g.Add(subject, IRI(RDFType), SysMLTerm("Package"))
		g.Add(wheel, IRI(RDFType), SysMLTerm("PartDefinition"))
		g.Add(subject, AnnotationJSONTerm(key), String(literal))
		_, err := ReconcileCollections(g)
		var refused *AnnotationError
		if !errors.As(err, &refused) || refused.Subject != subject.Value || refused.Key != key {
			t.Fatalf("json:%s %s was not refused as an AnnotationError: %v", key, literal, err)
		}
		if !strings.Contains(err.Error(), "appears at index") || !strings.Contains(err.Error(), "again at") {
			t.Errorf("the refusal of json:%s does not point at the repeated member: %v", key, err)
		}
	}
	// The same members once each are taken, whichever their order.
	g := NewGraph()
	g.Add(subject, IRI(RDFType), SysMLTerm("Package"))
	g.Add(subject, AnnotationJSONTerm("aliasIds"), String(`["b","a"]`))
	out, err := ReconcileCollections(g)
	if err != nil {
		t.Fatal(err)
	}
	if got := out.Objects(subject, SysML+"aliasIds"); len(got) != 2 || got[0] != String("b") || got[1] != String("a") {
		t.Errorf("distinct members were not all kept in annotation order: %v", got)
	}
}

// Literal members are spelled as the JSON value of their datatype: booleans and
// numbers bare, everything else as a string.
func TestCollectionJSONLiterals(t *testing.T) {
	text, err := CollectionJSON(IRI(Element+"P"), []Term{
		Bool(true), Int(3), TypedLiteral("2.50", XSD+"decimal"), String("a <b>"), TypedLiteral("2024-01-01", XSD+"date"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := `[true,3,2.50,"a <b>","2024-01-01"]`; text != want {
		t.Errorf("got %s, want %s", text, want)
	}
	if _, err := CollectionJSON(IRI(Element+"P"), []Term{TypedLiteral("three", XSD+"integer")}); err == nil {
		t.Error("a non-numeric integer literal was accepted")
	}
}

// A reference is spelled by its bare id within the subject's project scope and
// scope-qualified across scopes, and either spelling names its IRI back.
func TestReferenceIDCarriesTheScopeItCrosses(t *testing.T) {
	inP := ScopedElementIRIForID("acme.p", "shared")
	inQ := ScopedElementIRIForID("acme.q", "shared")
	root := ElementIRIForID("shared")
	cases := []struct {
		subject, target Term
		id              string
	}{
		{inP, inQ, "acme.q:shared"},
		{inP, inP, "shared"},
		{inP, root, ":shared"},
		{root, inQ, "acme.q:shared"},
		{root, root, "shared"},
		{inP, IRI(Expression + "acme.q:shared_p0"), "acme.q:shared_p0"},
	}
	for _, c := range cases {
		if got := ReferenceID(c.subject, c.target); got != c.id {
			t.Errorf("ReferenceID(%s, %s) = %q, want %q", c.subject, c.target, got, c.id)
		}
		if c.target.Value[:len(Element)] != Element {
			continue
		}
		if got := ReferenceIRI(c.subject, c.id); got != c.target {
			t.Errorf("ReferenceIRI(%s, %q) = %s, want %s", c.subject, c.id, got, c.target)
		}
	}
}

// ParseCollectionJSON reads references and primitives back and refuses anything
// the service does not store.
func TestParseCollectionJSON(t *testing.T) {
	members, err := ParseCollectionJSON(` [{"@id":"P__A"}, "s", true, 3, 2.50, 1e3] `)
	if err != nil {
		t.Fatal(err)
	}
	want := []CollectionMember{
		{ID: "P__A"},
		{Literal: String("s")},
		{Literal: Bool(true)},
		{Literal: TypedLiteral("3", XSD+"integer")},
		{Literal: TypedLiteral("2.50", XSD+"decimal")},
		{Literal: TypedLiteral("1e3", XSD+"double")},
	}
	if len(members) != len(want) {
		t.Fatalf("got %d members, want %d: %v", len(members), len(want), members)
	}
	for i := range want {
		if members[i] != want[i] {
			t.Errorf("member %d = %#v, want %#v", i, members[i], want[i])
		}
	}
	for name, text := range map[string]string{
		"an object":        `{"@id":"P__A"}`,
		"a string":         `"P__A"`,
		"two values":       `[{"@id":"P__A"}] [{"@id":"P__B"}]`,
		"trailing text":    `[{"@id":"P__A"}] x`,
		"not JSON":         `[{"@id":]`,
		"a null member":    `[null]`,
		"a nested array":   `[[{"@id":"P__A"}]]`,
		"an empty id":      `[{"@id":""}]`,
		"a wider object":   `[{"@id":"P__A","name":"A"}]`,
		"a foreign object": `[{"name":"A"}]`,
	} {
		if _, err := ParseCollectionJSON(text); err == nil {
			t.Errorf("%s was accepted: %s", name, text)
		}
	}
}
