package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
)

// idTurtle converts notation to Turtle, failing the test on error.
func idTurtle(t *testing.T, src string) []byte {
	t.Helper()
	out, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	return out
}

// toNotation converts Turtle back to notation, failing the test on error.
func toNotation(t *testing.T, turtle []byte) string {
	t.Helper()
	out, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v\n%s", err, turtle)
	}
	return string(out)
}

// roundTripsExactly asserts notation -> RDF -> notation reproduces the source
// byte for byte, and that the second hop is idempotent.
func roundTripsExactly(t *testing.T, src string) []byte {
	t.Helper()
	turtle := idTurtle(t, src)
	back := toNotation(t, turtle)
	if want := src; back != want {
		t.Errorf("round trip changed the notation:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
	second, err := export.Convert("m.sysml", []byte(back), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("second hop to turtle: %v", err)
	}
	if string(second) != string(turtle) {
		t.Errorf("second hop is not idempotent:\n--- first ---\n%s--- second ---\n%s", turtle, second)
	}
	return turtle
}

func TestAnnotatedIDRoundTrips(t *testing.T) {
	turtle := roundTripsExactly(t, `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; branch = "main"; org = "acme"; }
	part def Vehicle {
		@IdentityMetadata::ElementId { id = "8f3a41d0"; }
	}
}
`)
	text := string(turtle)
	for _, want := range []string{
		"<urn:sysmlv2:element:8f3a41d0>",
		`sysml:elementId "8f3a41d0"`,
		`sysx:declaredId "true"^^xsd:boolean`,
		`sysx:projectId "proj-1"`,
		`sysx:branch "main"`,
		`sysx:org "acme"`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("graph lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "MetadataUsage") {
		t.Errorf("identity annotations must be consumed, not exported as metadata content:\n%s", text)
	}
	// Memberships derive from the effective id, so they inherit its stability.
	if !strings.Contains(text, "8f3a41d0_om") {
		t.Errorf("membership id does not derive from the annotated id:\n%s", text)
	}
}

func TestDerivedIDRoundTrips(t *testing.T) {
	turtle := roundTripsExactly(t, `package P {
	part def Vehicle;
}
`)
	text := string(turtle)
	if !strings.Contains(text, `sysml:elementId "P__Vehicle"`) {
		t.Errorf("derived id is not the encoded qualified name:\n%s", text)
	}
	if strings.Contains(text, "declaredId") {
		t.Errorf("an unannotated element must not be marked declared:\n%s", text)
	}
}

// TestExplicitIDEqualToDerivedIDSurvives is the case explicitness exists for:
// the annotation equals the current encoding, and dropping it would turn the
// next rename into a delete plus create.
func TestExplicitIDEqualToDerivedIDSurvives(t *testing.T) {
	turtle := roundTripsExactly(t, `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def Vehicle {
		@IdentityMetadata::ElementId { id = "P__Vehicle"; }
	}
}
`)
	if !strings.Contains(string(turtle), `sysx:declaredId "true"^^xsd:boolean`) {
		t.Errorf("explicit id equal to the derived id lost its declaredness:\n%s", turtle)
	}
}

// TestRenameCorrelatesByID renames an annotated element between round trips:
// both graphs must address it by the same subject IRI.
func TestRenameCorrelatesByID(t *testing.T) {
	before := idTurtle(t, `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def Vehicle {
		@IdentityMetadata::ElementId { id = "8f3a41d0"; }
	}
}
`)
	after := idTurtle(t, `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def Car {
		@IdentityMetadata::ElementId { id = "8f3a41d0"; }
	}
}
`)
	const subject = "<urn:sysmlv2:element:8f3a41d0>"
	if !strings.Contains(string(before), subject) || !strings.Contains(string(after), subject) {
		t.Errorf("rename did not correlate by id:\n--- before ---\n%s--- after ---\n%s", before, after)
	}
	if !strings.Contains(string(after), `sysml:qualifiedName "P::Car"`) {
		t.Errorf("renamed element lost its new name:\n%s", after)
	}
}

// TestAdversarialIDsClassifyByType covers annotated ids that look like the
// ids this mapping derives for memberships and expression nodes: suffix
// parsing would misclassify them, rdf:type must not.
func TestAdversarialIDsClassifyByType(t *testing.T) {
	roundTripsExactly(t, `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def A {
		@IdentityMetadata::ElementId { id = "foreign_om"; }
	}
	part def B {
		@IdentityMetadata::ElementId { id = "left_pright"; }
	}
}
`)
}

// TestOldGraphStillReads strips the identity triples this change added: a
// graph from before it must read exactly as it did.
func TestOldGraphStillReads(t *testing.T) {
	src := `package P {
    part def Vehicle;
    part v : Vehicle;
}
`
	turtle := idTurtle(t, src)
	old := withoutTriples(t, withoutTriples(t, turtle, "sysml:elementId"), "sysx:declaredId")
	back, err := export.Convert("m.ttl", old, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("old graph did not read: %v\n%s", err, old)
	}
	if want := src; string(back) != want {
		t.Errorf("old graph read differently:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
}

func TestDanglingIDReferenceIsReported(t *testing.T) {
	turtle := `@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix sysml: <https://www.omg.org/spec/SysML#> .
@prefix elmt: <urn:sysmlv2:element:> .

elmt:P
    a sysml:Package ;
    sysml:qualifiedName "P" ;
    sysml:elementId "P" ;
    sysml:annotatedElement <urn:sysmlv2:element:missing> .
`
	_, err := export.Convert("m.ttl", []byte(turtle), export.FormatTurtle, export.FormatSysML)
	if err == nil {
		t.Fatal("a dangling id reference was not reported")
	}
	if !strings.Contains(err.Error(), `no element with id "missing"`) {
		t.Errorf("dangling reference reported without naming the missing id: %v", err)
	}
}

// TestMixedScopeQualifiesIRIs converts a workspace holding two project scopes:
// each scope's elements are qualified by its provenance, so ids repeated
// across scopes stay distinct subjects.
func TestMixedScopeQualifiesIRIs(t *testing.T) {
	turtle := idTurtle(t, `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; org = "acme"; }
	part def A {
		@IdentityMetadata::ElementId { id = "shared"; }
	}
}
package Q {
	@IdentityMetadata::ProjectRef { projectId = "proj-2"; org = "acme"; }
	part def B {
		@IdentityMetadata::ElementId { id = "shared"; }
	}
}
`)
	graph, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"urn:sysmlv2:element:acme.proj-1:shared",
		"urn:sysmlv2:element:acme.proj-2:shared",
	} {
		if got := rdf.LocalName(graph.Type(rdf.IRI(want))); got != "PartDefinition" {
			t.Errorf("scoped subject <%s> typed %q, want PartDefinition:\n%s", want, got, turtle)
		}
	}
}

// TestMixedScopeCollisionIsRefused puts two scopes of one project in one
// document with one id: their IRIs coincide, and merging two elements into
// one subject must be refused rather than silent.
func TestMixedScopeCollisionIsRefused(t *testing.T) {
	_, err := export.Convert("m.sysml", []byte(`package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; branch = "a"; }
	part def A {
		@IdentityMetadata::ElementId { id = "shared"; }
	}
}
package Q {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; branch = "b"; }
	part def B {
		@IdentityMetadata::ElementId { id = "shared"; }
	}
}
package R {
	@IdentityMetadata::ProjectRef { projectId = "proj-2"; }
}
`), export.FormatSysML, export.FormatTurtle)
	if err == nil {
		t.Fatal("two elements landing on one IRI were not refused")
	}
	if !strings.Contains(err.Error(), "lands on the same IRI") {
		t.Errorf("collision reported for the wrong reason: %v", err)
	}
}

// TestExpressionNodesInheritAnnotatedID checks that a value expression under
// an annotated element derives its node ids from the annotated id.
func TestExpressionNodesInheritAnnotatedID(t *testing.T) {
	turtle := roundTripsExactly(t, `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def A {
		attribute mass = 2 + 3 {
			@IdentityMetadata::ElementId { id = "stable-mass"; }
		}
	}
}
`)
	if !strings.Contains(string(turtle), "expr:stable-mass_pvalue") {
		t.Errorf("expression node id does not derive from the annotated owner id:\n%s", turtle)
	}
	// The structural half must survive without the verbatim text.
	stripped := withoutTriples(t, turtle, "sysx:sourceText")
	if _, err := export.Convert("m.ttl", stripped, export.FormatTurtle, export.FormatSysML); err != nil {
		t.Errorf("stripped graph did not read: %v", err)
	}
}

// TestUnevaluatedElementIDIsRefused rejects an annotation whose id the graph
// could not carry back, rather than silently dropping it.
func TestUnevaluatedElementIDIsRefused(t *testing.T) {
	_, err := export.Convert("m.sysml", []byte(`package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	attribute origin = "el-";
	part def A {
		@IdentityMetadata::ElementId { id = origin; }
	}
}
`), export.FormatSysML, export.FormatTurtle)
	if err == nil {
		t.Fatal("a non-constant ElementId was not refused")
	}
	if !strings.Contains(err.Error(), "ElementId annotation on P::A") {
		t.Errorf("refusal does not name the annotation: %v", err)
	}
}

// TestScopedGraphReadsBack reads a mixed-scope graph back to notation: the
// references resolve by id even though the IRIs carry scope qualifiers.
func TestScopedGraphReadsBack(t *testing.T) {
	roundTripsExactly(t, `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; org = "acme"; }
	part def A {
		@IdentityMetadata::ElementId { id = "shared"; }
	}
	part a : A;
}
package Q {
	@IdentityMetadata::ProjectRef { projectId = "proj-2"; org = "acme"; }
	part def B {
		@IdentityMetadata::ElementId { id = "shared"; }
	}
}
`)
}

// TestElementIDDiffersWithoutDeclaredID re-materializes the annotation when a
// graph states an id differing from the name's encoding but omits declaredId,
// as a foreign writer may.
func TestElementIDDiffersWithoutDeclaredID(t *testing.T) {
	turtle := idTurtle(t, `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def A {
		@IdentityMetadata::ElementId { id = "foreign-id"; }
	}
}
`)
	back := toNotation(t, withoutTriples(t, turtle, "sysx:declaredId"))
	if !strings.Contains(back, `@IdentityMetadata::ElementId { id = "foreign-id"; }`) {
		t.Errorf("differing id was not re-materialized without declaredId:\n%s", back)
	}
}

// TestUserElementWithLibraryIDKeepsItWithoutDeclaredID checks a foreign graph's
// user element that happens to carry a library uuid — Real's, or the one of
// Real's owning membership — keeps it as an annotation: the id is the norm's
// only on the library element it names.
func TestUserElementWithLibraryIDKeepsItWithoutDeclaredID(t *testing.T) {
	for _, id := range []string{
		"14c0aa22-5489-59b5-b438-ded26e83ba31", // ScalarValues::Real
		"ab72a695-5fe9-58a3-9d48-9e9a8711862d", // ScalarValues::Real's owning membership
	} {
		turtle := idTurtle(t, `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def A {
		@IdentityMetadata::ElementId { id = "`+id+`"; }
	}
}
`)
		back := toNotation(t, withoutTriples(t, turtle, "sysx:declaredId"))
		if !strings.Contains(back, `@IdentityMetadata::ElementId { id = "`+id+`"; }`) {
			t.Errorf("library uuid %s on P::A was not re-materialized without declaredId:\n%s", id, back)
		}
	}
}

// TestQualifiedNameKeyedGraphStillLinks locks the reader's identity key: a
// graph whose reference IRIs match only by element id, not byte-for-byte by
// IRI, still resolves.
func TestQualifiedNameKeyedGraphStillLinks(t *testing.T) {
	turtle := `@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix sysml: <https://www.omg.org/spec/SysML#> .

<urn:sysmlv2:element:P>
    a sysml:Package ;
    sysml:qualifiedName "P" ;
    sysml:elementId "P" .

<urn:example:other-base:stable-a>
    a sysml:PartDefinition ;
    sysml:qualifiedName "P::A" ;
    sysml:elementId "stable-a" ;
    sysml:owningNamespace <urn:sysmlv2:element:P> ;
    sysml:declaredName "A" .
`
	back, err := export.Convert("m.ttl", []byte(turtle), export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("graph with foreign IRI base did not read: %v", err)
	}
	if !strings.Contains(string(back), `@IdentityMetadata::ElementId { id = "stable-a"; }`) {
		t.Errorf("annotated id was not re-materialized:\n%s", back)
	}
}

// TestUnnamedElementKeepsAnnotatedID checks an unnamed declaration's
// annotated identity survives export: the table names it by symbol while the
// encoder positions it, and the two must still join.
func TestUnnamedElementKeepsAnnotatedID(t *testing.T) {
	turtle := idTurtle(t, `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def A;
	part {
		@IdentityMetadata::ElementId { id = "anon-id"; }
	}
}
`)
	graph, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatal(err)
	}
	subject := rdf.IRI("urn:sysmlv2:element:anon-id")
	if got := rdf.LocalName(graph.Type(subject)); got != "PartUsage" {
		t.Fatalf("subject anon-id typed %q, want PartUsage", got)
	}
	back := toNotation(t, turtle)
	if !strings.Contains(back, `@IdentityMetadata::ElementId { id = "anon-id"; }`) {
		t.Errorf("unnamed element's annotated id was not re-materialized:\n%s", back)
	}
	second, err := export.Convert("m.sysml", []byte(back), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("second hop to turtle: %v", err)
	}
	if string(second) != string(turtle) {
		t.Errorf("second hop is not idempotent:\n--- first ---\n%s--- second ---\n%s", turtle, second)
	}
}

// TestAnnotatedUnnamedSuccessionEndRoundTrips checks a positional succession
// end bound to an unnamed annotated member points at the member's declared
// identity, so the link does not dangle when the graph is read back.
func TestAnnotatedUnnamedSuccessionEndRoundTrips(t *testing.T) {
	turtle := idTurtle(t, `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	state def S;
	state def M {
		state : S {
			@IdentityMetadata::ElementId { id = "entry-id"; }
		}
		then s1;
		state s1 : S;
	}
}
`)
	if !strings.Contains(string(turtle), "sysx:sourceMember elmt:entry-id") {
		t.Errorf("the positional end does not point at the declared id:\n%s", turtle)
	}
	back := toNotation(t, turtle)
	if !strings.Contains(back, "then s1;") {
		t.Fatalf("the succession beside the annotated unnamed entry should come back:\n%s", back)
	}
	if !strings.Contains(back, `@IdentityMetadata::ElementId { id = "entry-id"; }`) {
		t.Errorf("the unnamed end's annotated id was not re-materialized:\n%s", back)
	}
}

// TestAboutFormElementIDRoundTrips checks an `about`-form ElementId is
// consumed into identity like an inline one: the target's subject carries the
// declared id, and the notation built from the graph alone has the annotation
// inline, while the source text keeps the form it was written in.
func TestAboutFormElementIDRoundTrips(t *testing.T) {
	src := `package P {
    @IdentityMetadata::ProjectRef { projectId = "proj-1"; }
    part def A;
    metadata : IdentityMetadata::ElementId about A { id = "stable-a"; }
}
`
	turtle := idTurtle(t, src)
	graph, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatal(err)
	}
	subject := rdf.IRI("urn:sysmlv2:element:stable-a")
	if got := rdf.LocalName(graph.Type(subject)); got != "PartDefinition" {
		t.Fatalf("subject stable-a typed %q, want PartDefinition", got)
	}
	if back := toNotation(t, turtle); back != src {
		t.Errorf("the about-form annotation did not come back as written:\n%s", back)
	}
	back := toNotation(t, withoutTriples(t, turtle, "sysx:sourceText"))
	if !strings.Contains(back, `@IdentityMetadata::ElementId { id = "stable-a"; }`) {
		t.Errorf("about-form id was not re-materialized:\n%s", back)
	}
	if strings.Contains(back, "about") {
		t.Errorf("consumed about-form annotation leaked back into the notation:\n%s", back)
	}
	// Inlining the annotation gives the element a body, so the notation —
	// not the first graph — is the fixed point the second hop must reach.
	second, err := export.Convert("m.sysml", []byte(back), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("second hop to turtle: %v", err)
	}
	if again := toNotation(t, second); again != back {
		t.Errorf("second hop is not idempotent:\n--- first ---\n%s--- second ---\n%s", back, again)
	}
}

// TestDeclaredIDCollidingWithMembershipIsRefused declares an id equal to the
// membership id minted for another element: both live in the element
// namespace, and merging them would corrupt the graph.
func TestDeclaredIDCollidingWithMembershipIsRefused(t *testing.T) {
	_, err := export.Convert("m.sysml", []byte(`package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def A {
		@IdentityMetadata::ElementId { id = "stable"; }
	}
	part def B {
		@IdentityMetadata::ElementId { id = "stable_om"; }
	}
}
`), export.FormatSysML, export.FormatTurtle)
	if err == nil {
		t.Fatal("an id landing on a membership IRI was not refused")
	}
	if !strings.Contains(err.Error(), "lands on the same IRI") {
		t.Errorf("collision reported for the wrong reason: %v", err)
	}
}

// TestDeclaredIDLikeExpressionNodeStaysDisjoint declares an id spelled like
// another element's expression-node id: expression nodes live in their own
// namespace, so both round-trip without merging.
func TestDeclaredIDLikeExpressionNodeStaysDisjoint(t *testing.T) {
	turtle := roundTripsExactly(t, `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	attribute a = 1 {
		@IdentityMetadata::ElementId { id = "stable"; }
	}
	part def B {
		@IdentityMetadata::ElementId { id = "stable_pvalue"; }
	}
}
`)
	graph, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatal(err)
	}
	if got := rdf.LocalName(graph.Type(rdf.IRI("urn:sysmlv2:element:stable_pvalue"))); got != "PartDefinition" {
		t.Errorf("element stable_pvalue typed %q, want PartDefinition", got)
	}
	if rdf.LocalName(graph.Type(rdf.IRI("urn:opensysml:expr:stable_pvalue"))) == "PartDefinition" {
		t.Error("expression node merged with the element sharing its spelling")
	}
}

// TestMembershipIDsAreDisjointFromAdversarialElements checks the graph a
// foreign-looking id produces still separates the element from the membership
// minted for it.
func TestMembershipIDsAreDisjointFromAdversarialElements(t *testing.T) {
	turtle := idTurtle(t, `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; }
	part def A {
		@IdentityMetadata::ElementId { id = "foreign_om"; }
	}
}
`)
	graph, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatal(err)
	}
	subject := rdf.IRI("urn:sysmlv2:element:foreign_om")
	if got := rdf.LocalName(graph.Type(subject)); got != "PartDefinition" {
		t.Errorf("element with id foreign_om typed %q, want PartDefinition", got)
	}
	membership := rdf.IRI("urn:sysmlv2:element:foreign_om_om")
	if got := rdf.LocalName(graph.Type(membership)); got != "OwningMembership" {
		t.Errorf("membership of foreign_om typed %q, want OwningMembership", got)
	}
}

// TestLibraryElementsCarryNormativeIDs pins the ids the norm fixes: a bundled
// library file converts to the UUIDs the pilot assigns its elements and their
// owning memberships, states them as implied rather than declared, and a copy
// of the same notation that is not the library file keeps encoded ids.
func TestLibraryElementsCarryNormativeIDs(t *testing.T) {
	const name = "Kernel Libraries/Kernel Data Type Library/ScalarValues.kerml"
	src, err := libs.EmbeddedSource().Read(name)
	if err != nil {
		t.Fatal(err)
	}
	turtle, err := export.Convert(name, src, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	text := string(turtle)
	for _, want := range []string{
		"<urn:sysmlv2:element:40bb440c-5036-58e1-8675-5afccb8b8f1d>",
		`sysml:elementId "40bb440c-5036-58e1-8675-5afccb8b8f1d"`,
		"<urn:sysmlv2:element:14c0aa22-5489-59b5-b438-ded26e83ba31>",
		`sysml:elementId "14c0aa22-5489-59b5-b438-ded26e83ba31"`,
		"elmt:ab72a695-5fe9-58a3-9d48-9e9a8711862d",
		`sysml:elementId "ab72a695-5fe9-58a3-9d48-9e9a8711862d"`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("graph lacks %q", want)
		}
	}
	for _, reject := range []string{"declaredId", "ScalarValues__Real"} {
		if strings.Contains(text, reject) {
			t.Errorf("graph states %q, which the norm's ids leave no place for", reject)
		}
	}
	// The library file comes back as written, so a second hop states the same ids.
	back := toNotation(t, turtle)
	if back != string(src) {
		t.Errorf("the library file did not come back as written:\n%s", back)
	}
	second, err := export.Convert(name, []byte(back), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("second hop to turtle: %v", err)
	}
	if string(second) != text {
		t.Errorf("second hop is not idempotent:\n%s", second)
	}
	// Read back from the mapping alone, the ids the norm implies stay implied.
	if back := toNotation(t, withoutSourceText(t, turtle)); strings.Contains(back, "ElementId") {
		t.Errorf("the ids the norm implies came back as declared annotations:\n%s", back)
	}
	edited := append([]byte("// not the library\n"), src...)
	turtle, err = export.Convert(name, edited, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("edited copy to turtle: %v", err)
	}
	if text := string(turtle); !strings.Contains(text, "elmt:ScalarValues__Real") ||
		strings.Contains(text, "14c0aa22-5489-59b5-b438-ded26e83ba31") {
		t.Errorf("an edited copy of a library file is not the library, so its ids are encoded names:\n%s", text)
	}
}

// A redefining feature without a name of its own takes the redefined one, so
// the norm fixes an id for it while the graph names it by position. Neither
// side may mistake that id for a declared one.
func TestEffectivelyNamedLibraryMemberCarriesNormativeID(t *testing.T) {
	const name = "Domain Libraries/Geometry/ShapeItems.sysml"
	src, err := libs.EmbeddedSource().Read(name)
	if err != nil {
		t.Fatal(err)
	}
	turtle, err := export.Convert(name, src, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatalf("parse turtle: %v", err)
	}
	// ShapeItems::Polyhedron::edges under the SysML prefix.
	const edges = "1c6076b4-48fa-5c5f-81f2-7c850aee33b1"
	subject := rdf.ElementIRIForID(edges)
	if got, _ := graph.Object(subject, rdf.SysML+"qualifiedName"); got != rdf.String("ShapeItems::Polyhedron::@3") {
		t.Fatalf("the redefining item is named %v in the graph", got)
	}
	if graph.HasProperty(subject, rdf.OpenSysML+"declaredId") {
		t.Errorf("the norm's id is stated as declared")
	}
	if back := toNotation(t, withoutSourceText(t, turtle)); strings.Contains(back, "ElementId") {
		t.Errorf("the ids the norm implies came back as declared annotations:\n%s", back)
	}
}
