package queryexec

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// ownedElementIDs is the metaclass path of each element root owns, by its elementId,
// requiring every element to have one of its own.
func ownedElementIDs(t *testing.T, fixture executionFixture, root string) map[string]string {
	t.Helper()
	read := export.NewElementIDReader([]export.ElementIDDocument{{File: fixture.file, Root: fixture.root}})
	ids := map[string]string{}
	seen := map[*symbols.Symbol]bool{}
	var walk func(sym *symbols.Symbol, path string)
	walk = func(sym *symbols.Symbol, path string) {
		if seen[sym] {
			return
		}
		seen[sym] = true
		here := path + "/"
		if metaclass := fixture.model.MetaclassOf(sym); metaclass != nil {
			here += metaclass.Name
		}
		id, err := ElementIDOf(read, fixture.index, sym)
		switch {
		case err != nil || id == "":
			t.Errorf("%s: elementId %q, %v; want one", here, id, err)
		case ids[id] != "":
			t.Errorf("%s: elementId %q is also that of %s", here, id, ids[id])
		default:
			ids[id] = here
		}
		for _, rel := range fixture.model.OwnedRelationships(sym) {
			walk(rel, here)
		}
		if related, ok := fixture.model.ReflectiveElements(sym, "ownedRelatedElement"); ok {
			for _, element := range related {
				walk(element, here)
			}
		}
	}
	walk(fixture.symbol(t, root), "")
	return ids
}

// TestCrossSubsettingAndItsChainHaveElementIDs: `crosses a.q` reads the ids the conversion
// mints for the CrossSubsetting, the chain Feature it owns and the chain's FeatureChainings.
func TestCrossSubsettingAndItsChainHaveElementIDs(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
	package P {
		part def D;
		part def A { part q : D; }
		part def B { part r : A; }
		connection def C {
			end part a : A crosses b.r;
			end part b : B => a.q;
		}
		connection def E {
			end part a : A;
			end part b : B crosses a;
		}
	}
}`)
	ids := ownedElementIDs(t, fixture, "P")
	const end = "/Package/OwningMembership/ConnectionDefinition/FeatureMembership/PartUsage"
	for id, want := range map[string]string{
		"Observatory__P__C__a_cs":           end + "/CrossSubsetting",
		"Observatory__P__C__a_pchain0":      end + "/CrossSubsetting/Feature",
		"Observatory__P__C__a_pchain0_pfc0": end + "/CrossSubsetting/Feature/FeatureChaining",
		"Observatory__P__C__a_pchain0_pfc1": end + "/CrossSubsetting/Feature/FeatureChaining",
		"Observatory__P__C__b_cs":           end + "/CrossSubsetting",
		"Observatory__P__C__b_pchain0":      end + "/CrossSubsetting/Feature",
		"Observatory__P__C__b_pchain0_pfc0": end + "/CrossSubsetting/Feature/FeatureChaining",
		"Observatory__P__C__b_pchain0_pfc1": end + "/CrossSubsetting/Feature/FeatureChaining",
		"Observatory__P__E__b_cs":           end + "/CrossSubsetting",
	} {
		if got := ids[id]; got != want {
			t.Errorf("elementId %q identifies %q, want %s", id, got, want)
		}
	}
}

// TestTypeOperandRelationshipsHaveElementIDs: each operand of `unions`, `intersects` and
// `differences` reads the id the conversion mints for its relationship, indexed by operand.
func TestTypeOperandRelationshipsHaveElementIDs(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
	package P {
		classifier A { feature f : B; }
		classifier B;
		classifier X unions A, B;
		classifier Y intersects A, B;
		classifier Z differences A, B;
		feature a : A;
		feature w unions a.f, a;
	}
}`)
	ids := ownedElementIDs(t, fixture, "P")
	for id, want := range map[string]string{
		"Observatory__P__X_un0": "/Unioning", "Observatory__P__X_un1": "/Unioning",
		"Observatory__P__Y_is0": "/Intersecting", "Observatory__P__Y_is1": "/Intersecting",
		"Observatory__P__Z_df0": "/Differencing", "Observatory__P__Z_df1": "/Differencing",
		"Observatory__P__w_un0": "/Unioning", "Observatory__P__w_un1": "/Unioning",
		"Observatory__P__w_pchain0":      "/Unioning/Feature",
		"Observatory__P__w_pchain0_pfc0": "/Unioning/Feature/FeatureChaining",
		"Observatory__P__w_pchain0_pfc1": "/Unioning/Feature/FeatureChaining",
	} {
		if got := ids[id]; !strings.HasSuffix(got, want) {
			t.Errorf("elementId %q identifies %q, want a path ending in %s", id, got, want)
		}
	}
}

// TestDisjoiningInvertingAndFeaturingClausesHaveElementIDs: each target of `disjoint from`,
// `inverse of` and `featured by` reads the id the conversion mints for its relationship.
func TestDisjoiningInvertingAndFeaturingClausesHaveElementIDs(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
	package P {
		classifier A { feature q : B; }
		classifier B;
		classifier X disjoint from A, B;
		feature f : A;
		feature g : B inverse of f;
		feature h featured by A, B;
		feature k disjoint from f.q;
		feature m featured by A;
	}
}`)
	ids := ownedElementIDs(t, fixture, "P")
	for id, want := range map[string]string{
		"Observatory__P__X_dj0": "/Disjoining", "Observatory__P__X_dj1": "/Disjoining",
		"Observatory__P__g_iv0":          "/FeatureInverting",
		"Observatory__P__h_tf0":          "/TypeFeaturing",
		"Observatory__P__h_tf1":          "/TypeFeaturing",
		"Observatory__P__m_tf0":          "/TypeFeaturing",
		"Observatory__P__k_dj0":          "/Disjoining",
		"Observatory__P__k_pchain0":      "/Disjoining/Feature",
		"Observatory__P__k_pchain0_pfc0": "/Disjoining/Feature/FeatureChaining",
		"Observatory__P__k_pchain0_pfc1": "/Disjoining/Feature/FeatureChaining",
	} {
		if got := ids[id]; !strings.HasSuffix(got, want) {
			t.Errorf("elementId %q identifies %q, want a path ending in %s", id, got, want)
		}
	}
}
