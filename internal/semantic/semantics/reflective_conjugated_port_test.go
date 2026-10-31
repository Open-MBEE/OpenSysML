package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// A port definition owns the ConjugatedPortDefinition it implies as its first
// member, and the conjugate owns one PortConjugation back (SysML 8.3.11.3).
func TestPortDefinitionOwnsItsConjugate(t *testing.T) {
	m, root := buildModelWithStdlib(t, `
		package P {
			port def <q> Q { attribute x; }
			port def <s>;
			part def R;
		}
	`)
	pkg, _ := root.LookupLocal("P")
	q := member(t, m, pkg, "Q")
	r := member(t, m, pkg, "R")

	memberships := m.OwnedMemberships(q)
	if len(memberships) != 2 {
		t.Fatalf("owned memberships of Q = %d, want the conjugate's then x's", len(memberships))
	}
	membership := memberships[0]
	conjugate := membership.Implicit.Member
	if conjugate == nil || !conjugate.IsConjugatedPortDefinition() || !conjugate.Implied() {
		t.Fatalf("first membership's member = %v, want the implied conjugate", conjugate)
	}
	if conjugate.Name != "~Q" || conjugate.ShortName != "~q" {
		t.Errorf("conjugate named %q <%s>, want ~Q <~q>", conjugate.Name, conjugate.ShortName)
	}
	if got := m.MetaclassOf(conjugate); got == nil || got.Name != "ConjugatedPortDefinition" {
		t.Errorf("conjugate metaclass = %v, want ConjugatedPortDefinition", got)
	}
	if got := m.MetaclassOf(membership); got == nil || got.Name != "OwningMembership" {
		t.Errorf("membership metaclass = %v, want OwningMembership", got)
	}
	if got, _ := m.ReflectiveElements(q, "ownedMember"); len(got) != 2 || got[0] != conjugate {
		t.Errorf("ownedMember of Q = %v, want the conjugate first", got)
	}
	if got, ok := m.LookupMember(q, "~Q"); !ok || got != conjugate {
		t.Errorf("Q::'~Q' resolves to %v, want the conjugate", got)
	}

	rels := m.OwnedRelationships(conjugate)
	if len(rels) != 1 {
		t.Fatalf("owned relationships of ~Q = %d, want its PortConjugation", len(rels))
	}
	conjugation := rels[0]
	if got := m.MetaclassOf(conjugation); got == nil || got.Name != "PortConjugation" {
		t.Errorf("conjugation metaclass = %v, want PortConjugation", got)
	}
	for feature, want := range map[string]*symbols.Symbol{
		"source": conjugate, "conjugatedType": conjugate, "owningRelatedElement": conjugate,
		"target": q, "originalType": q, "originalPortDefinition": q,
	} {
		if got, ok := m.ReflectiveElements(conjugation, feature); !ok || len(got) != 1 || got[0] != want {
			t.Errorf("%s of the conjugation = %v, want %s", feature, got, want.Name)
		}
	}
	if got, ok := m.ReflectiveElements(conjugation, "relatedElement"); !ok || len(got) != 2 || got[0] != conjugate || got[1] != q {
		t.Errorf("relatedElement of the conjugation = %v, want [~Q Q]", got)
	}
	if got, ok := m.ReflectiveElements(conjugate, "ownedConjugator"); !ok || len(got) != 1 || got[0] != conjugation {
		t.Errorf("ownedConjugator of ~Q = %v, want its PortConjugation", got)
	}
	if got, ok := m.ReflectiveFeatureValue(conjugate, "isConjugated"); !ok || !got.Bool {
		t.Errorf("isConjugated of ~Q = %v, %v; want true", got, ok)
	}
	if got, ok := m.ReflectiveFeatureValue(q, "isConjugated"); !ok || got.Bool {
		t.Errorf("isConjugated of Q = %v, %v; want false", got, ok)
	}
	if symbols.KeyOf(q) == symbols.KeyOf(conjugate) || symbols.KeyOf(q).String() == symbols.KeyOf(conjugate).String() {
		t.Errorf("Q and ~Q share the element key %s", symbols.KeyOf(q))
	}
	if got := m.OwnedMemberships(conjugate); len(got) != 0 {
		t.Errorf("the conjugate owns %d memberships, want none", len(got))
	}
	short, ok := pkg.Scope.LookupLocal("s")
	if !ok {
		t.Fatal("port def <s> not found")
	}
	if got := m.OwnedMemberships(short); len(got) != 1 || got[0].Implicit.Member.ShortName != "~s" || got[0].Implicit.Member.Name != "~s" {
		t.Errorf("a short-named port definition owns %v, want its conjugate <~s>", got)
	}
	if got := m.OwnedMemberships(r); len(got) != 0 {
		t.Errorf("a part definition owns %d memberships, want none", len(got))
	}
}
