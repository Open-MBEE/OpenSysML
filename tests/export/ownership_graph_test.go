package export_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
)

// ownershipModel nests packages, definitions, usages and a state's entry action,
// so it covers ownership by a namespace and ownership by a relationship.
const ownershipModel = `package Outer {
    package Inner {
        part def Vehicle {
            attribute mass;
            private part wheel;
        }
        state def Modes {
            entry action warm;
            state off;
        }
    }
}
`

// Every element carries the id the SysML v2 API addresses it by, which is the id
// its own IRI ends in: paged listing selects on it and a query rewrites an @id
// constraint to it, so an element without one is invisible to both.
func TestEveryElementCarriesItsElementID(t *testing.T) {
	graph := turtleOf(t, "ownership", ownershipModel)
	for _, subject := range graph.Subjects() {
		if !strings.HasPrefix(subject.Value, rdf.Element) {
			continue
		}
		id, ok := graph.Lexical(subject, rdf.SysML+"elementId")
		if !ok {
			t.Errorf("<%s> states no sysml:elementId", subject.Value)
			continue
		}
		if want := strings.TrimPrefix(subject.Value, rdf.Element); id != want {
			t.Errorf("<%s> states elementId %q, want %q", subject.Value, id, want)
		}
	}
}

// The roots endpoint takes an element with no owner and no owning related
// element for a root, so only the outermost package may leave both unstated.
func TestOnlyTheOutermostElementIsARoot(t *testing.T) {
	graph := turtleOf(t, "ownership", ownershipModel)
	var roots []string
	for _, subject := range graph.Subjects() {
		if !strings.HasPrefix(subject.Value, rdf.Element) {
			continue
		}
		_, owned := graph.Object(subject, rdf.SysML+"owner")
		_, related := graph.Object(subject, rdf.SysML+"owningRelatedElement")
		if !owned && !related {
			roots = append(roots, subject.Value)
		}
	}
	if len(roots) != 1 || roots[0] != rdf.ElementIRI("Outer").Value {
		t.Errorf("roots = %v, want only <%s>", roots, rdf.ElementIRI("Outer").Value)
	}
}

// A client walking down from a root reaches every member through the membership
// the abstract syntax puts between them, which states both ends.
func TestMembersAreReachedThroughTheirMembership(t *testing.T) {
	graph := turtleOf(t, "ownership", ownershipModel)
	for _, member := range []string{"Outer::Inner", "Outer::Inner::Vehicle", "Outer::Inner::Vehicle::mass"} {
		owner := rdf.ElementIRI(member[:strings.LastIndex(member, "::")])
		membership := rdf.OwningMembershipIRI(member)
		element := rdf.ElementIRI(member)
		for _, want := range []struct {
			subject   rdf.Term
			property  string
			object    rdf.Term
			collected bool
		}{
			{element, "owner", owner, false},
			{element, "owningMembership", membership, false},
			{membership, "memberElement", element, false},
			{membership, "ownedMemberElement", element, false},
			{membership, "owningRelatedElement", owner, false},
			{membership, "membershipOwningNamespace", owner, false},
			{owner, "ownedMember", element, true},
			{owner, "ownedMembership", membership, true},
			{owner, "ownedRelationship", membership, true},
		} {
			objects := graph.Objects(want.subject, rdf.SysML+want.property)
			found := false
			for _, object := range objects {
				found = found || object == want.object
			}
			if !found {
				t.Errorf("<%s> does not state %s <%s>, only %v", want.subject.Value, want.property, want.object.Value, objects)
			}
			if !want.collected && len(objects) != 1 {
				t.Errorf("<%s> states %d values for %s, want 1", want.subject.Value, len(objects), want.property)
			}
		}
	}
	if typ := graph.Type(rdf.OwningMembershipIRI("Outer::Inner")); typ != rdf.SysML+"OwningMembership" {
		t.Errorf("the membership is typed %q, want an OwningMembership", typ)
	}
}

// A type owns a feature through a FeatureMembership, which is what the API's
// payloads carry for a usage nested in a definition.
func TestAFeatureIsOwnedThroughAFeatureMembership(t *testing.T) {
	graph := turtleOf(t, "ownership", ownershipModel)
	owner := rdf.ElementIRI("Outer::Inner::Vehicle")
	feature := rdf.ElementIRI("Outer::Inner::Vehicle::mass")
	membership := rdf.OwningMembershipIRI("Outer::Inner::Vehicle::mass")
	if typ := graph.Type(membership); typ != rdf.SysML+"FeatureMembership" {
		t.Errorf("the membership is typed %q, want a FeatureMembership", typ)
	}
	for _, want := range []struct {
		subject  rdf.Term
		property string
		object   rdf.Term
	}{
		{membership, "ownedMemberFeature", feature},
		{membership, "owningType", owner},
		{owner, "ownedFeature", feature},
		{owner, "ownedFeatureMembership", membership},
	} {
		found := false
		for _, object := range graph.Objects(want.subject, rdf.SysML+want.property) {
			found = found || object == want.object
		}
		if !found {
			t.Errorf("<%s> does not state %s <%s>", want.subject.Value, want.property, want.object.Value)
		}
	}
}

// A relationship owns its related element directly, so a state's entry action
// hangs off the membership that states the entry rather than off a minted one.
func TestARelationshipOwnsItsMemberDirectly(t *testing.T) {
	graph := turtleOf(t, "ownership", ownershipModel)
	entry := rdf.ElementIRI("Outer::Inner::Modes::@0")
	action := rdf.ElementIRI("Outer::Inner::Modes::@0::warm")
	if owner, ok := graph.Object(action, rdf.SysML+"owner"); !ok || owner != entry {
		t.Errorf("the entry action's owner is <%s>, want the entry membership <%s>", owner.Value, entry.Value)
	}
	if owned, ok := graph.Object(entry, rdf.SysML+"ownedRelatedElement"); !ok || owned != action {
		t.Errorf("the entry membership does not own the action directly, it owns <%s>", owned.Value)
	}
	if _, minted := graph.Object(rdf.OwningMembershipIRI("Outer::Inner::Modes::@0::warm"), rdf.SysML+"memberElement"); minted {
		t.Error("a membership was minted between the entry membership and the action it owns")
	}
}

// The visibility a member is declared with is stated by its membership, which is
// where the metamodel declares the property, and comes back from there.
func TestVisibilityIsStatedByTheMembership(t *testing.T) {
	graph := turtleOf(t, "ownership", ownershipModel)
	membership := rdf.OwningMembershipIRI("Outer::Inner::Vehicle::wheel")
	if vis, ok := graph.Lexical(membership, rdf.SysML+"visibility"); !ok || vis != "private" {
		t.Errorf("the membership states visibility %q (present=%t), want private", vis, ok)
	}
	if vis, ok := graph.Lexical(rdf.ElementIRI("Outer::Inner::Vehicle::wheel"), rdf.SysML+"visibility"); ok {
		t.Errorf("the member itself states visibility %q", vis)
	}
}

// The containment tree comes back from the memberships alone: without the source
// text the heads were written as, and without the compact owningNamespace triple
// that a graph built from the abstract syntax would not carry.
func TestOwnershipComesBackFromTheMembershipsAlone(t *testing.T) {
	turtle, err := export.Convert("m.sysml", []byte(ownershipModel), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	withText, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	stripped := turtle
	for _, property := range []string{"sysx:sourceText", "sysml:owningNamespace", "sysml:owner"} {
		stripped = withoutTriples(t, stripped, property)
	}
	if strings.Contains(string(stripped), "owningNamespace") {
		t.Fatal("the compact ownership triple was not stripped")
	}
	back, err := export.Convert("m.ttl", stripped, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation from the memberships alone: %v", err)
	}
	if string(back) != string(withText) {
		t.Errorf("the memberships did not carry the tree\n--- with source text ---\n%s\n--- from the graph ---\n%s", withText, back)
	}
	again, err := export.Convert("m.sysml", back, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle again: %v", err)
	}
	if string(again) != string(turtle) {
		t.Errorf("the second hop changed the graph\n--- first ---\n%s\n--- second ---\n%s", turtle, again)
	}
}

// elementSideOwnership are the properties an element states its owner with; a
// membership states the same edge from its own side.
var elementSideOwnership = []string{"sysx:sourceText", "sysml:owningNamespace", "sysml:owner", "sysml:owningMembership", "sysml:owningRelationship"}

// A graph built from the abstract syntax may state ownership from the membership
// alone — sysml:membershipOwningNamespace and sysml:memberElement, with no
// inverse on the member — and the tree comes back from that. An entry action is
// owned by a membership that is an element in its own right, which no membership
// edge states, so it alone floats to the root.
func TestOwnershipComesBackFromTheMembershipSideAlone(t *testing.T) {
	turtle, err := export.Convert("m.sysml", []byte(ownershipModel), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	stripped := turtle
	for _, property := range elementSideOwnership {
		stripped = withoutTriples(t, stripped, property)
	}
	back, err := export.Convert("m.ttl", stripped, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation from the membership side alone: %v", err)
	}
	const want = `package Outer {
    package Inner {
        part def Vehicle {
            attribute mass;
            private part wheel;
        }
        state def Modes {
            entry;
            state off;
        }
    }
}
action warm;
`
	if string(back) != want {
		t.Errorf("the memberships did not carry the tree\n--- want ---\n%s\n--- got ---\n%s", want, back)
	}
}

// The negative control for the tests above: with every ownership property gone
// from the elements and the memberships gone with them, the tree flattens, which
// is what makes those properties load-bearing.
func TestWithoutAnyOwnershipPropertyTheTreeFlattens(t *testing.T) {
	turtle, err := export.Convert("m.sysml", []byte(ownershipModel), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	stripped := withoutMemberships(turtle)
	for _, property := range elementSideOwnership {
		stripped = withoutTriples(t, stripped, property)
	}
	back, err := export.Convert("m.ttl", stripped, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if !strings.HasPrefix(string(back), "package Outer {\n}\n") {
		t.Errorf("the package should have lost its members, since nothing states them:\n%s", back)
	}
}

// withoutMemberships drops the subjects that state ownership rather than a
// declaration: the memberships this tool mints, whose IRIs end in _om.
func withoutMemberships(turtle []byte) []byte {
	var kept []string
	for _, block := range strings.Split(string(turtle), "\n\n") {
		if head, _, _ := strings.Cut(block, "\n"); strings.HasSuffix(head, "_om") {
			continue
		}
		kept = append(kept, block)
	}
	return []byte(strings.Join(kept, "\n\n"))
}

// A graph in the compact shape this tool wrote before memberships were
// materialized still converts: ownership falls back to owningNamespace.
func TestCompactOwnershipGraphStillConverts(t *testing.T) {
	const compact = `@prefix elmt: <urn:sysmlv2:element:> .
@prefix sysml: <https://www.omg.org/spec/SysML#> .
@prefix sysx: <urn:opensysml:sysml:> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

elmt:Outer
    a sysml:Package ;
    sysml:qualifiedName "Outer" ;
    sysx:memberIndex "0"^^xsd:integer ;
    sysml:declaredName "Outer" ;
    sysx:hasBody "true"^^xsd:boolean .

elmt:Outer__Vehicle
    a sysml:PartDefinition ;
    sysml:qualifiedName "Outer::Vehicle" ;
    sysml:owningNamespace elmt:Outer ;
    sysx:memberIndex "0"^^xsd:integer ;
    sysml:visibility "private" ;
    sysml:declaredName "Vehicle" ;
    sysx:hasBody "false"^^xsd:boolean .
`
	back, err := export.Convert("compact.ttl", []byte(compact), export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("a graph in the compact shape should still convert: %v", err)
	}
	if want := "package Outer {\n    private part def Vehicle;\n}\n"; string(back) != want {
		t.Errorf("compact graph converted to:\n%s\nwant:\n%s", back, want)
	}
}

// A membership that states neither of its ends is reported rather than dropped,
// since the member it owns would silently leave the tree.
func TestMembershipWithoutItsEndsIsReported(t *testing.T) {
	const broken = `@prefix elmt: <urn:sysmlv2:element:> .
@prefix sysml: <https://www.omg.org/spec/SysML#> .
@prefix sysx: <urn:opensysml:sysml:> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

elmt:Outer
    a sysml:Package ;
    sysml:qualifiedName "Outer" ;
    sysml:elementId "Outer" ;
    sysx:memberIndex "0"^^xsd:integer ;
    sysml:declaredName "Outer" ;
    sysx:hasBody "true"^^xsd:boolean ;
    sysml:ownedMembership elmt:Outer__Vehicle_om .

elmt:Outer__Vehicle_om
    a sysml:OwningMembership ;
    sysml:elementId "Outer__Vehicle_om" ;
    sysml:membershipOwningNamespace elmt:Outer .
`
	_, err := export.Convert("broken.ttl", []byte(broken), export.FormatTurtle, export.FormatSysML)
	if err == nil {
		t.Fatal("a membership that owns nothing should be reported")
	}
	if !strings.Contains(err.Error(), "sysml:memberElement") {
		t.Errorf("the report should name the property it needs: %v", err)
	}
}

// A membership whose end names no element of the graph is reported rather than
// dropped: the owner would be written without the member it owns.
func TestMembershipWithAnAbsentEndIsReported(t *testing.T) {
	const graph = `@prefix sysml: <https://www.omg.org/spec/SysML#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
@prefix ex: <urn:example:> .

ex:P a sysml:Package ; sysml:qualifiedName "P" ; sysml:declaredName "P" .
ex:C a sysml:CalculationDefinition ; sysml:qualifiedName "P::C" ; sysml:declaredName "C" ; sysml:owningMembership ex:C_m .
ex:C_m a sysml:OwningMembership ; sysml:membershipOwningNamespace ex:P ; sysml:memberElement ex:C .
ex:r_m a sysml:ResultExpressionMembership ; sysml:membershipOwningNamespace ex:C ; sysml:ownedMemberFeature ex:r ; sysml:ownedResultExpression ex:r .
ex:r a sysml:OperatorExpression ; sysml:operator "+" ; sysml:argument ex:r_a, ex:r_b .
ex:r_a a sysml:LiteralInteger ; sysml:value "2"^^xsd:integer .
ex:r_b a sysml:LiteralInteger ; sysml:value "3"^^xsd:integer .
`
	if _, err := export.Convert("m.ttl", []byte(graph), export.FormatTurtle, export.FormatSysML); err != nil {
		t.Fatalf("the intact graph should convert: %v", err)
	}
	for _, tc := range []struct{ end, from, to, want string }{
		{"member", "sysml:ownedMemberFeature ex:r ; sysml:ownedResultExpression ex:r .", "sysml:ownedMemberFeature ex:gone ; sysml:ownedResultExpression ex:gone .", "its member <urn:example:gone> is not an element of the graph"},
		{"owning namespace", "sysml:membershipOwningNamespace ex:C ; sysml:ownedMemberFeature", "sysml:membershipOwningNamespace ex:nowhere ; sysml:ownedMemberFeature", "its owning namespace <urn:example:nowhere> is not an element of the graph"},
	} {
		broken := strings.Replace(graph, tc.from, tc.to, 1)
		if broken == graph {
			t.Fatalf("the %s was not rewritten", tc.end)
		}
		_, err := export.Convert("m.ttl", []byte(broken), export.FormatTurtle, export.FormatSysML)
		var unsupported *export.UnsupportedError
		if !errors.As(err, &unsupported) {
			t.Fatalf("a membership with an absent %s should be an UnsupportedError, got %v", tc.end, err)
		}
		if !strings.Contains(err.Error(), "the membership <urn:example:r_m>") || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("for an absent %s:\n got %v\nwant %s", tc.end, err, tc.want)
		}
	}
}
