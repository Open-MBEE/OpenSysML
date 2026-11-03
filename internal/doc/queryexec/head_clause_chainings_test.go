package queryexec

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// TestHeadClauseChainFeatureOwnsItsChainings: the chain Feature a head clause's chain target denotes
// (`part b :> a.x`) owns its FeatureChainings at ownedFeatureChaining, identified under the head (chain0).
func TestHeadClauseChainFeatureOwnsItsChainings(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
	part def A { part x; }
	part a : A;
	part b :> a.x;
}`)
	read := export.NewElementIDReader([]export.ElementIDDocument{{File: fixture.file, Root: fixture.root}})
	rdfClass := apiJSONClasses(t, fixture)
	b := fixture.symbol(t, "b")
	bID, _ := ElementIDOf(read, fixture.index, b)
	rels := fixture.model.ImplicitRelationships(b)
	if len(rels) != 1 {
		t.Fatalf("b owns %d relationships, want its Subsetting", len(rels))
	}
	targets, _ := fixture.model.ReflectiveElements(rels[0], "target")
	if len(targets) != 1 || targets[0].Chain == nil {
		t.Fatalf("target = %v, want the chain Feature", targets)
	}
	chain := targets[0]
	chainID, err := ElementIDOf(read, fixture.index, chain)
	if err != nil || chainID != bID+"_pchain0" || rdfClass[chainID] != "Feature" {
		t.Fatalf("chain Feature id %q (%v), class %q; want %s_pchain0, Feature", chainID, err, rdfClass[chainID], bID)
	}
	owned, _ := fixture.model.ReflectiveElements(chain, "ownedFeatureChaining")
	all, _ := fixture.model.ReflectiveElements(chain, "ownedRelationship")
	if len(owned) != 2 || len(all) != 2 || owned[0] != all[0] || owned[1] != all[1] {
		t.Fatalf("ownedFeatureChaining %v, ownedRelationship %v, want the same two FeatureChainings", owned, all)
	}
	for i, chaining := range owned {
		id, err := ElementIDOf(read, fixture.index, chaining)
		want := chainID + "_pfc" + string(rune('0'+i))
		if err != nil || id != want || rdfClass[want] != "FeatureChaining" {
			t.Errorf("chaining %d: id %q (%v), written as %q; want %q FeatureChaining", i, id, err, rdfClass[want], want)
		}
	}
}
