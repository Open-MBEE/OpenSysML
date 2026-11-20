package export

import "testing"

// An action body's `first a if g then b;` is a TransitionUsage owning its source membership, link parameter,
// guard, succession and transitionLink (SysML.xtext GuardedSuccession), in the symbol structure as in the graph.
func TestGuardedSuccessionOwnsItsTransitionPartsInBothStructures(t *testing.T) {
	diff, ok := differentialOf("p.sysml", []byte(`package P {
		action def A {
			attribute g : ScalarValues::Boolean;
			action a;
			action b;
			first a if g then b;
			succession s first a if g then b;
		}
	}`))
	if !ok {
		t.Fatal("the model has no differential")
	}
	if len(diff.unmatched) > 0 {
		t.Errorf("graph elements the symbol structure lacks: %v", diff.unmatched)
	}
	for _, property := range []string{
		"Element::ownedElement",
		"Element::ownedRelationship",
		"Namespace::ownedMembership",
		"Namespace::member",
		"Type::ownedFeature",
	} {
		counts := diff.counts[property]
		if counts == nil || counts.equal == 0 {
			t.Errorf("%s was not compared", property)
			continue
		}
		if counts.different+counts.missing+counts.extra > 0 {
			t.Errorf("%s: %+v, samples %v", property, *counts, diff.samples[property])
		}
	}
}
