package export

import "testing"

// A constraint stated as a chain alone (`require h.r1;`) reference-subsets the chain Feature in both
// structures, as a head clause's chain does (KerML 8.3.3.3.9); neither targets the chain's last feature.
func TestShorthandChainReferenceTargetsTheChainInBothStructures(t *testing.T) {
	diff, ok := differentialOf("p.sysml", []byte(`package P {
		requirement h { requirement r1; }
		requirement Spec { require h.r1; }
		requirement Other { assume h.r1; }
		part a { part x; }
		part b :> a.x;
	}`))
	if !ok {
		t.Fatal("the model has no differential")
	}
	if len(diff.unmatched) > 0 {
		t.Errorf("graph elements the symbol structure lacks: %v", diff.unmatched)
	}
	for _, property := range []string{"Relationship::target", "Relationship::ownedRelatedElement", "Relationship::relatedElement", "Element::ownedElement"} {
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
