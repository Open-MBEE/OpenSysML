package export_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
)

// collections is a model whose graph states two collections the decoder reads
// back: C's specializations and D's suppliers.
const collections = `package P {
    part def A;
    part def B;
    part def C :> A, B;
    dependency D from A to B, C;
}
`

// A collection is written twice: as typed triples and as the JSON annotation
// Flexo reads, holding the same members in the same order.
func TestCollectionsAreAnnotated(t *testing.T) {
	turtle := string(idTurtle(t, collections))
	for _, want := range []string{
		"@prefix json: <urn:sysmlv2:annotation:json:> .",
		"    sysml:specializes elmt:P__A, elmt:P__B ;",
		`    json:specializes "[{\"@id\":\"P__A\"},{\"@id\":\"P__B\"}]"`,
		"    sysml:supplier elmt:P__B, elmt:P__C ;",
		`    json:supplier "[{\"@id\":\"P__B\"},{\"@id\":\"P__C\"}]"`,
		`    json:ownedMember "[{\"@id\":\"P__A\"},{\"@id\":\"P__B\"},{\"@id\":\"P__C\"}]"`,
	} {
		if !strings.Contains(turtle, want) {
			t.Errorf("missing %q in:\n%s", want, turtle)
		}
	}
	for _, single := range []string{"json:client", "json:declaredName", "json:owner "} {
		if strings.Contains(turtle, single) {
			t.Errorf("a single-valued property was annotated as %s:\n%s", single, turtle)
		}
	}
}

// The three spellings of a collection read back alike: the annotation alone, as
// a Flexo-produced graph states it; the typed triples alone, as our older graphs
// and other tools state it; or both, when they agree.
func TestCollectionSpellingsReadAlike(t *testing.T) {
	turtle := structural(t, idTurtle(t, collections))
	annotationOnly := withoutTriples(t, withoutTriples(t, []byte(turtle), "sysml:specializes"), "sysml:supplier")
	triplesOnly := withoutTriples(t, withoutTriples(t, []byte(turtle), "json:specializes"), "json:supplier")
	if strings.Contains(string(annotationOnly), "sysml:specializes") || !strings.Contains(string(triplesOnly), "sysml:specializes") {
		t.Fatalf("the fixtures were not derived:\n%s\n%s", annotationOnly, triplesOnly)
	}
	want := toNotation(t, []byte(turtle))
	if !strings.Contains(want, "part def C specializes A, B;") || !strings.Contains(want, "dependency D from A to B, C;") {
		t.Fatalf("the structural graph lost a collection:\n%s", want)
	}
	for name, form := range map[string][]byte{"annotation only": annotationOnly, "typed triples only": triplesOnly} {
		if back := toNotation(t, form); back != want {
			t.Errorf("%s reads differently:\n--- want ---\n%s--- got ---\n%s", name, want, back)
		}
		// Whatever the spelling read, the graph written again states both.
		if again := structural(t, idTurtle(t, toNotation(t, form))); again != turtle {
			t.Errorf("%s does not write back the same graph:\n--- want ---\n%s--- got ---\n%s", name, turtle, again)
		}
	}
}

// When both spellings state the same members, the annotation carries the order.
func TestAnnotationOrderWinsOverTypedTriples(t *testing.T) {
	turtle := editTurtle(t, idTurtle(t, collections),
		"    sysml:specializes elmt:P__A, elmt:P__B ;",
		"    sysml:specializes elmt:P__B, elmt:P__A ;")
	turtle = []byte(structural(t, turtle))
	if back := toNotation(t, turtle); !strings.Contains(back, "part def C specializes A, B;") {
		t.Errorf("the annotation's order was not kept:\n%s", back)
	}
}

// Two spellings that disagree are refused, naming the subject and the property,
// rather than one of them being picked.
func TestConflictingCollectionSpellingsAreRefused(t *testing.T) {
	turtle := editTurtle(t, idTurtle(t, collections),
		"    sysml:specializes elmt:P__A, elmt:P__B ;",
		"    sysml:specializes elmt:P__A ;")
	_, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	var conflict *rdf.CollectionConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("want a CollectionConflictError, got %v", err)
	}
	if conflict.Subject != "urn:sysmlv2:element:P__C" || conflict.Key != "specializes" {
		t.Errorf("the error names <%s> sysml:%s, want <urn:sysmlv2:element:P__C> sysml:specializes", conflict.Subject, conflict.Key)
	}
	for _, want := range []string{"<urn:sysmlv2:element:P__C>", "sysml:specializes", `{"@id":"P__B"}`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the message lacks %q: %v", want, err)
		}
	}
}

// An annotation that is not one JSON-array literal is refused with a typed error.
func TestMalformedCollectionAnnotationsAreRefused(t *testing.T) {
	annotation := `    json:specializes "[{\"@id\":\"P__A\"},{\"@id\":\"P__B\"}]" .`
	for name, edited := range map[string]string{
		"not JSON":       `    json:specializes "[{" .`,
		"not an array":   `    json:specializes "{\"@id\":\"P__A\"}" .`,
		"a null member":  `    json:specializes "[null]" .`,
		"two literals":   `    json:specializes "[{\"@id\":\"P__A\"},{\"@id\":\"P__B\"}]", "[]" .`,
		"not a literal":  `    json:specializes elmt:P__A .`,
		"a typed number": `    json:specializes "1"^^xsd:integer .`,
	} {
		t.Run(name, func(t *testing.T) {
			turtle := editTurtle(t, idTurtle(t, collections), annotation, edited)
			_, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
			var unsupported *export.UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Fatalf("want an UnsupportedError, got %v", err)
			}
			if !strings.Contains(err.Error(), "json:specializes") || !strings.Contains(err.Error(), "<urn:sysmlv2:element:P__C>") {
				t.Errorf("the error does not name the annotation and its subject: %v", err)
			}
		})
	}
}

// An annotation names members by id alone; in a graph of two project scopes that
// reuse an id, the reference resolves to the member in the referrer's own scope.
func TestAnnotatedReferencesResolveWithinScope(t *testing.T) {
	turtle := structural(t, idTurtle(t, `package P {
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
	part b : B;
}
`))
	if !strings.Contains(turtle, `json:ownedMember "[{\"@id\":\"shared\"},`) {
		t.Fatalf("the fixture does not annotate a collection holding a reused id:\n%s", turtle)
	}
	annotationOnly := []byte(turtle)
	for _, key := range []string{"sysml:ownedMember", "sysml:ownedMembership", "sysml:ownedRelationship"} {
		annotationOnly = withoutTriples(t, annotationOnly, key)
	}
	want := toNotation(t, []byte(turtle))
	if back := toNotation(t, annotationOnly); back != want {
		t.Errorf("the annotation-only graph reads differently:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
}

// A reference into another project scope is spelled with that scope's qualifier in
// the annotation as in the typed triple, and reads back from the annotation alone.
func TestAnnotatedReferencesReachAcrossScopes(t *testing.T) {
	turtle := structural(t, idTurtle(t, `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; org = "acme"; }
	part def A :> Q::B, Q::C;
}
package Q {
	@IdentityMetadata::ProjectRef { projectId = "proj-2"; org = "acme"; }
	part def B {
		@IdentityMetadata::ElementId { id = "b-id"; }
	}
	part def C;
}
`))
	if !strings.Contains(turtle, "sysml:specializes elmt:acme.proj-2:b-id, elmt:acme.proj-2:Q__C ;") {
		t.Fatalf("the fixture does not reference across scopes:\n%s", turtle)
	}
	if !strings.Contains(turtle, `json:specializes "[{\"@id\":\"acme.proj-2:b-id\"},{\"@id\":\"acme.proj-2:Q__C\"}]"`) {
		t.Fatalf("the annotation does not qualify the references it makes across scopes:\n%s", turtle)
	}
	want := toNotation(t, []byte(turtle))
	back := toNotation(t, withoutTriples(t, []byte(turtle), "sysml:specializes"))
	if back != want || !strings.Contains(back, "part def A specializes Q::B, Q::C;") {
		t.Errorf("the annotation-only graph reads differently:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
}

// A reference across scopes to an id the referrer's own scope also carries names the
// other scope's element, from the annotation alone as from the typed triple; a bare id
// annotation for it disagrees with the typed triple and is refused.
func TestAnnotatedCrossScopeReferenceIsNotRetargetedLocally(t *testing.T) {
	turtle := structural(t, idTurtle(t, `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; org = "acme"; }
	part def A {
		@IdentityMetadata::ElementId { id = "shared"; }
	}
	part def D :> Q::B, P::A;
}
package Q {
	@IdentityMetadata::ProjectRef { projectId = "proj-2"; org = "acme"; }
	part def B {
		@IdentityMetadata::ElementId { id = "shared"; }
	}
}
`))
	annotation := `json:specializes "[{\"@id\":\"acme.proj-2:shared\"},{\"@id\":\"shared\"}]"`
	if !strings.Contains(turtle, "sysml:specializes elmt:acme.proj-2:shared, elmt:acme.proj-1:shared ;") || !strings.Contains(turtle, annotation) {
		t.Fatalf("the fixture does not reference the same id in both scopes:\n%s", turtle)
	}
	want := toNotation(t, []byte(turtle))
	back := toNotation(t, withoutTriples(t, []byte(turtle), "sysml:specializes"))
	if back != want || !strings.Contains(back, "part def D specializes Q::B, A;") {
		t.Errorf("the annotation-only graph reads differently:\n--- want ---\n%s--- got ---\n%s", want, back)
	}
	bare := editTurtle(t, []byte(turtle), annotation, `json:specializes "[{\"@id\":\"shared\"},{\"@id\":\"acme.proj-1:shared\"}]"`)
	var conflict *rdf.CollectionConflictError
	if _, err := export.Convert("m.ttl", bare, export.FormatTurtle, export.FormatSysML); !errors.As(err, &conflict) || conflict.Key != "specializes" {
		t.Errorf("a bare id for a cross-scope reference was not refused as a conflict: %v", err)
	}
}

// An element may declare the id an expression node derives, since the two live in
// different namespaces. An annotation's bare id names either; a collection stated by
// the annotation alone resolves it in the referrer's namespace — an expression node's
// arguments to the node, an element's members to the element.
func TestAnnotatedReferencesResolveInTheReferrersNamespace(t *testing.T) {
	const model = `package P {
    attribute x = f(1, 2, 3);
    part def A {
        @IdentityMetadata::ElementId { id = "P__x_pvalue_pa0"; }
    }
}
`
	turtle := string(idTurtle(t, model))
	for _, want := range []string{
		`json:argument "[{\"@id\":\"P__x_pvalue_pa0\"},{\"@id\":\"P__x_pvalue_pa1\"},{\"@id\":\"P__x_pvalue_pa2\"}]"`,
		`json:ownedMember "[{\"@id\":\"P__x\"},{\"@id\":\"P__x_pvalue_pa0\"}]"`,
		"expr:P__x_pvalue_pa0\n    a sysml:LiteralInteger ;",
		"elmt:P__x_pvalue_pa0\n    a sysml:PartDefinition ;",
	} {
		if !strings.Contains(turtle, want) {
			t.Fatalf("the fixture lacks %q:\n%s", want, turtle)
		}
	}
	structured := []byte(structural(t, []byte(turtle)))
	annotationOnly := withoutTriples(t, withoutTriples(t, structured, "sysml:argument"), "sysml:ownedMember")
	if back := toNotation(t, annotationOnly); back != model {
		t.Errorf("the annotation-only graph reads back as:\n%s\nwant:\n%s", back, model)
	}
}

// A reference the annotation alone states to an element the graph does not
// describe is refused as a dangling reference, as a typed triple's would be.
func TestAnnotatedReferenceToAnAbsentElementIsRefused(t *testing.T) {
	turtle := editTurtle(t, idTurtle(t, collections),
		`    json:specializes "[{\"@id\":\"P__A\"},{\"@id\":\"P__B\"}]" .`,
		`    json:specializes "[{\"@id\":\"P__A\"},{\"@id\":\"P__Z\"}]" .`)
	turtle = withoutTriples(t, []byte(structural(t, turtle)), "sysml:specializes")
	_, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
	var unsupported *export.UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("want an UnsupportedError, got %v", err)
	}
	if !strings.Contains(err.Error(), "P__Z") {
		t.Errorf("the error does not name the absent element: %v", err)
	}
}
