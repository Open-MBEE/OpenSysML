package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

func TestReflectiveTransitionOwnsItsDottedSourceChain(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
    state def S {
        state a { state b; state c; }
        transition t first a.b then a.c;
    }
}`)
	trans := nestedSym(t, root, "P::S::t")
	var chain, membership *symbols.Symbol
	for _, own := range m.OwnedMemberships(trans) {
		if member := own.Implicit.Member; member != nil && member.Chain != nil {
			chain, membership = member, own
		}
		if own.Implicit.Form == symbols.ImplicitReferenceMembership && own.Implicit.Slot == "sourcemember" {
			t.Errorf("transition owns a source Membership for a dotted source; want the chain Feature")
		}
	}
	if chain == nil {
		t.Fatal("transition owns no chain Feature for `first a.b`")
	}
	if mc := m.MetaclassOf(membership); mc == nil || mc.Name != "OwningMembership" {
		t.Errorf("the chain's membership is a %v, want OwningMembership", mc)
	}
	if mc := m.MetaclassOf(chain); mc == nil || mc.Name != "Feature" {
		t.Errorf("the source chain is a %v, want Feature", mc)
	}
	if got := m.OwningRelationship(chain); got != membership {
		t.Errorf("the chain's owningRelationship is %v, want its membership", got)
	}
	chained := reflectiveElementsOf(t, m, chain, "chainingFeature")
	if len(chained) != 2 || chained[0].Name != "a" || chained[1].Name != "b" {
		t.Errorf("chainingFeature = %v, want [a b]", fqns(chained))
	}
	if chainings := reflectiveElementsOf(t, m, chain, "ownedFeatureChaining"); len(chainings) != 2 {
		t.Errorf("ownedFeatureChaining has %d members, want 2", len(chainings))
	}
}
