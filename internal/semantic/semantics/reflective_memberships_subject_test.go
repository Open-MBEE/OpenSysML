package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A satisfy usage's `by` clause declares a subject parameter: an unnamed
// ReferenceUsage owned through a SubjectMembership (SysML 8.3.19), which the
// reflective read lists among the usage's owned relationships.
func TestSatisfySubjectMembershipIsOwned(t *testing.T) {
	const src = `package P {
		part def Sys;
		requirement def R { subject s : Sys; }
		part sys : Sys {
			satisfy requirement r : R by sys {
				require constraint { true }
			}
			satisfy requirement r2 : R by sys;
		}
	}`
	m, root := buildModelWithLibraryNamedKind(t, "satisfy.sysml", source.KindSysML, src)
	p, _ := root.LookupLocal("P")
	sys, _ := p.Scope.LookupLocal("sys")
	satisfy, ok := sys.Scope.LookupLocal("r")
	if !ok {
		t.Fatal("P::sys::r not found")
	}
	var membership *symbols.Symbol
	for _, rel := range m.OwnedRelationships(satisfy) {
		if rel.Implicit != nil && rel.Implicit.Form == symbols.ImplicitMembership && isSatisfySubjectSymbol(rel.Implicit.Member) {
			membership = rel
		}
	}
	if membership == nil {
		t.Fatalf("no subject membership among %d owned relationships", len(m.OwnedRelationships(satisfy)))
	}
	if got := m.implicitMetaclassName(membership); got != "SubjectMembership" {
		t.Errorf("membership metaclass = %q, want SubjectMembership", got)
	}
	subject := membership.Implicit.Member
	if got := sysmlMetaclassName(subject); got != "ReferenceUsage" {
		t.Errorf("subject metaclass = %q, want ReferenceUsage", got)
	}
	if owner := m.ownerOf(subject); owner != satisfy {
		t.Errorf("subject owner = %v, want the satisfy usage", owner)
	}
	if got := m.OwningMembership(subject); got != membership {
		t.Errorf("subject owning membership = %v, want the subject membership", got)
	}
	if got := m.OwnedMemberships(satisfy); len(got) != 2 || got[0] != membership {
		// The subject precedes the body's required constraint, as declared.
		t.Errorf("owned memberships = %v, want the subject membership first of 2", got)
	}
	if got := m.SubjectParameterOf(satisfy); got != subject {
		t.Errorf("subject parameter = %v, want the by subject", got)
	}
	if got := ownedMembersOf(satisfy); len(got) != 2 || got[0] != subject {
		t.Errorf("owned members = %v, want the subject first of 2", got)
	}
	if got := m.ImplicitRoleRedefinitions(subject); len(got) != 1 || got[0] == nil || got[0].Name != "s" {
		t.Errorf("subject redefines %v, want R's subject s", got)
	}
	if got := m.FeatureTypeSet(subject); len(got) != 1 || got[0].Name != "Sys" {
		t.Errorf("subject types = %v, want [Sys] through the redefined subject", got)
	}
	// The subject is a referential usage of the satisfy usage, an Occurrence: it varies in
	// time, and its `by` value is the FeatureValue it owns, as the conversion writes them.
	if !m.UsageMayTimeVary(subject) || !m.FeatureIsVariable(subject) || !m.UsageMayTimeVaryForOwningType(subject, satisfy) {
		t.Errorf("subject mayTimeVary = %v (for its owning type %v), isVariable = %v, want true", m.UsageMayTimeVary(subject), m.UsageMayTimeVaryForOwningType(subject, satisfy), m.FeatureIsVariable(subject))
	}
	if got, ok := m.ReflectiveFeatureValue(subject, "isVariable"); !ok || got.Kind != symbols.FilterValueBool || !got.Bool {
		t.Errorf("isVariable of the subject = %+v (ok %v), want true", got, ok)
	}
	value := m.ValueExpression(subject)
	if value == nil {
		t.Fatal("the subject has no value expression for its `by sys`")
	}
	if got := m.OwnedMemberships(subject); len(got) != 1 || got[0].Implicit == nil || got[0].Implicit.Member != value || m.implicitMetaclassName(got[0]) != "FeatureValue" {
		t.Errorf("subject owned memberships = %v, want one FeatureValue owning its value", got)
	}
	r2, _ := sys.Scope.LookupLocal("r2")
	other := m.SubjectParameterOf(r2)
	if other == nil || other == subject || other.DeclSpan == subject.DeclSpan || other.DeclSpan == (source.Span{}) {
		t.Errorf("r2's subject %v must be its own, with its own non-empty span (r's: %v)", other, subject.DeclSpan)
	}
}
