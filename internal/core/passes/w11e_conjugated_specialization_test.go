package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
)

// A conjugated type inverts the directions of its features, so specializing it
// is meaningless (KerML validateSpecializationSpecificNotConjugated).
func TestConjugatedTypeIsNotSpecialized(t *testing.T) {
	const src = `package p {
		classifier A;
		classifier B;
		classifier C conjugates A;
		subtype C specializes B;
	}`
	if !hasMessage(diagsIn(t, "a.kerml", src, "type"), msgConjugatedSpecific) {
		t.Fatalf("expected %q", msgConjugatedSpecific)
	}
}

// The rule shares the type tier with the specialization metaclass rules, so a
// metaclass error elsewhere in the file does not suppress it.
func TestConjugatedRuleSurvivesAMetaclassError(t *testing.T) {
	const src = `package p {
		classifier A;
		classifier B;
		classifier C conjugates A;
		subtype C specializes B;
		datatype D1;
		class C1;
		datatype D2 specializes D1, C1;
	}`
	diags := diagsIn(t, "a.kerml", src, "type")
	if !hasMessage(diags, msgConjugatedSpecific) {
		t.Fatalf("expected %q alongside the metaclass error, got %v", msgConjugatedSpecific, diags)
	}
	if !hasMessage(diags, msgW11ASpecializeClassOrAssoc) {
		t.Fatalf("expected %q, got %v", msgW11ASpecializeClassOrAssoc, diags)
	}
}

// Subsetting, redefinition and feature typing are Specializations too, so a
// conjugated feature as their specific end is reported like a subclassifier.
func TestConjugatedFeatureIsNotSpecializedThroughAnyRelationshipMember(t *testing.T) {
	for _, member := range []string{
		"specialization subset g subsets h;",
		"specialization redefinition g redefines h;",
		"specialization typing g typed by A;",
		"specialization subtype g :> h;",
	} {
		src := `package p {
			class A;
			feature f : A; feature g ~ f; feature h : A;
			` + member + `
		}`
		if !hasMessage(diagsIn(t, "a.kerml", src, "type"), msgConjugatedSpecific) {
			t.Errorf("%s: expected %q", member, msgConjugatedSpecific)
		}
	}
}

// Only the type that owns the conjugation is conjugated: a standalone
// conjugation member, a conjugate's owned feature, and an unrelated feature
// specialize freely.
func TestConjugationDoesNotSpreadToOtherSpecifics(t *testing.T) {
	const src = `package p {
		class A; class B; class Z ~ A;
		conjugation conjugate B conjugates A;
		specialization subtype B specializes A;
		class C ~ Z { feature x : A; }
		specialization subtype C::x :> Z;
		feature f : A; feature h : A;
		specialization subset h subsets f;
	}`
	if diags := diagsIn(t, "a.kerml", src, "type"); hasMessage(diags, msgConjugatedSpecific) {
		t.Fatalf("unexpected %q in %v", msgConjugatedSpecific, diags)
	}
}

func hasMessage(diags []diag.Diagnostic, msg string) bool {
	for _, d := range diags {
		if d.Message == msg {
			return true
		}
	}
	return false
}
