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
		requirement def R;
		part def Sys;
		part sys : Sys {
			satisfy requirement r : R by sys;
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
	if n := len(m.OwnedRelationships(satisfy)); n != 2 {
		// The requirement's ReferenceSubsetting and the subject membership.
		t.Errorf("%d owned relationships, want 2", n)
	}
}
