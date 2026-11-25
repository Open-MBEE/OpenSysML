package queryexec

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// TestFlowEndChainFeatureHasItsOwnElementID: the chain Feature a flow's end references ahead of its payload
// (`a.p` of `flow from a.p.q to b.p.q`) and its FeatureChainings are identified as the conversion writes them.
func TestFlowEndChainFeatureHasItsOwnElementID(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
	part def S { port p { out item q; } }
	part def T {
		part a : S;
		part b : S;
		flow f from a.p.q to b.p.q;
	}
}`)
	read := export.NewElementIDReader([]export.ElementIDDocument{{File: fixture.file, Root: fixture.root}})
	rdfClass := apiJSONClasses(t, fixture)
	flow := fixture.symbol(t, "T::f")
	flowID, _ := ElementIDOf(read, fixture.index, flow)
	related, _ := fixture.model.ReflectiveElements(flow, "relatedFeature")
	if len(related) != 2 {
		t.Fatalf("relatedFeature = %v, want the chains a.p and b.p", related)
	}
	for i, chain := range related {
		want := flowID + "_pend" + string(rune('0'+i)) + "_pchain"
		chainID, err := ElementIDOf(read, fixture.index, chain)
		if err != nil || chainID != want || rdfClass[want] != "Feature" {
			t.Fatalf("end %d: chain Feature id %q (%v), written as %q; want %q Feature", i, chainID, err, rdfClass[want], want)
		}
		owned, _ := fixture.model.ReflectiveElements(chain, "ownedFeatureChaining")
		if len(owned) != 2 {
			t.Fatalf("end %d: ownedFeatureChaining = %v, want two FeatureChainings", i, owned)
		}
		for link, chaining := range owned {
			want := chainID + "_pfc" + string(rune('0'+link))
			id, err := ElementIDOf(read, fixture.index, chaining)
			if err != nil || id != want || rdfClass[want] != "FeatureChaining" {
				t.Errorf("end %d chaining %d: id %q (%v), written as %q; want %q FeatureChaining", i, link, id, err, rdfClass[want], want)
			}
		}
	}
}
