package queryexec

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// TestSynthesizedPartsHaveTheirOwnElementIDs: a send's argument parameters, a written multiplicity
// range and a transition's guard are identified, with what each owns, as the conversion mints them.
func TestSynthesizedPartsHaveTheirOwnElementIDs(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
	package P {
		part def W;
		part def T { port p; }
		part x : W [1..2];
		action def A {
			in w : W; in t : T;
			action n send w;
			action m send w to t;
			action o send w via t.p to t;
		}
		state def S { state s1; state s2; transition g first s1 if true then s2; }
	}
}`)
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
	walk(fixture.symbol(t, "P"), "")
	const send, value = "/Package/OwningMembership/ActionDefinition/FeatureMembership/SendActionUsage", "/ParameterMembership/ReferenceUsage/FeatureValue"
	const transition = "/Package/OwningMembership/StateDefinition/FeatureMembership/TransitionUsage"
	for id, want := range map[string]string{
		"Observatory__P__x_pmultiplicity":                  "/Package/OwningMembership/PartUsage/OwningMembership/MultiplicityRange",
		"Observatory__P__x_pmultiplicity_om":               "/Package/OwningMembership/PartUsage/OwningMembership",
		"Observatory__P__A__n___400":                       send + "/ParameterMembership/ReferenceUsage",
		"Observatory__P__A__n___400_om":                    send + "/ParameterMembership",
		"Observatory__P__A__n___400_pvalue":                send + value + "/FeatureReferenceExpression",
		"Observatory__P__A__n___400_pvalue_om":             send + value,
		"Observatory__P__A__n___400_pvalue_preferent":      send + value + "/FeatureReferenceExpression/Membership",
		"Observatory__P__A__n___400_pvalue_pout":           send + value + "/FeatureReferenceExpression/ReturnParameterMembership/Feature",
		"Observatory__P__A__n___400_pvalue_pout_om":        send + value + "/FeatureReferenceExpression/ReturnParameterMembership",
		"Observatory__P__A__m___401":                       send + "/ParameterMembership/ReferenceUsage",
		"Observatory__P__A__m___402_pvalue":                send + value + "/FeatureReferenceExpression",
		"Observatory__P__A__o___401_pvalue":                send + value + "/FeatureChainExpression",
		"Observatory__P__A__o___401_pvalue_ptargetFeature": send + value + "/FeatureChainExpression/Membership",
		"Observatory__P__A__o___402_pvalue_om":             send + value,
		"Observatory__P__S__g_pguard":                      transition + "/TransitionFeatureMembership/LiteralBoolean",
		"Observatory__P__S__g_pguard_om":                   transition + "/TransitionFeatureMembership",
	} {
		if got := ids[id]; got != want {
			t.Errorf("elementId %q identifies %q, want %s", id, got, want)
		}
	}
}
