package queryexec

import (
	"encoding/json"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// TestConnectorEndChainFeatureOwnsItsChainings: the chain Feature a chain end denotes (`connect a.o to
// b.i`) owns one FeatureChaining per link, identified as the conversion mints them, under the chain at fc%d.
func TestConnectorEndChainFeatureOwnsItsChainings(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
	part def A { port o; }
	part def B { port i; }
	part a : A;
	part b : B;
	connection c connect a.o to b.i;
}`)
	read := export.NewElementIDReader([]export.ElementIDDocument{{File: fixture.file, Root: fixture.root}})
	rdfClass := apiJSONClasses(t, fixture)
	ends, _ := fixture.model.ReflectiveElements(fixture.symbol(t, "c"), "connectorEnd")
	if len(ends) != 2 {
		t.Fatalf("connectorEnd = %d ends, want 2", len(ends))
	}
	links := [][]string{{"a", "o"}, {"b", "i"}}
	for i, end := range ends {
		targets, _ := fixture.model.ReflectiveElements(fixture.model.ImplicitRelationships(end)[0], "target")
		chain := targets[0]
		chainID, _ := ElementIDOf(read, fixture.index, chain)
		owned, ok := fixture.model.ReflectiveElements(chain, "ownedRelationship")
		if !ok || len(owned) != 2 {
			t.Fatalf("end %d: chain Feature owns %d relationships (%v), want 2 FeatureChainings", i, len(owned), ok)
		}
		for j, chaining := range owned {
			if got := fixture.model.MetaclassOf(chaining).Name; got != "FeatureChaining" {
				t.Errorf("end %d chaining %d: metaclass %s, want FeatureChaining", i, j, got)
			}
			features, _ := fixture.model.ReflectiveElements(chaining, "chainingFeature")
			if len(features) != 1 || features[0].Name != links[i][j] {
				t.Errorf("end %d chaining %d: chainingFeature %v, want %s", i, j, features, links[i][j])
			}
			id, err := ElementIDOf(read, fixture.index, chaining)
			want := chainID + "_pfc" + string(rune('0'+j))
			if err != nil || id != want {
				t.Errorf("end %d chaining %d: id %q (%v), want %q", i, j, id, err, want)
			}
			if rdfClass[want] != "FeatureChaining" {
				t.Errorf("the conversion writes %q as %q, want FeatureChaining", want, rdfClass[want])
			}
		}
	}
}

// apiJSONClasses is the @type of every element the conversion writes for the fixture, by @id.
func apiJSONClasses(t *testing.T, fixture executionFixture) map[string]string {
	t.Helper()
	graph, err := export.ToRDF(fixture.file, fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	data, err := export.WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	var objects []map[string]any
	if err := json.Unmarshal(data, &objects); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, object := range objects {
		out[object["@id"].(string)], _ = object["@type"].(string)
	}
	return out
}
