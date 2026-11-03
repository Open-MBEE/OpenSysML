package queryexec

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// TestSatisfySubjectValueNamesItsReferentOnDemand: a satisfy subject's `by` value owns the Membership
// to the feature it names, and a chained value's chain Feature its FeatureChainings, before any
// document walk resolved the clause — as a query over the model reads them.
func TestSatisfySubjectValueNamesItsReferentOnDemand(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
	part def Sys { part sub : Sys; }
	requirement def R;
	requirement massRequirement : R;
	part telescope : Sys;
	part observatory { satisfy massRequirement by telescope; }
	part site { satisfy massRequirement by telescope.sub; }
}`)
	model := fixture.model
	valueOf := func(owner string) *symbols.Symbol {
		for _, membership := range model.OwnedRelationships(fixture.symbol(t, owner)) {
			satisfies, _ := model.ReflectiveElements(membership, "ownedRelatedElement")
			for _, satisfy := range satisfies {
				for _, rel := range model.OwnedRelationships(satisfy) {
					if model.MetaclassOf(rel).Name != "SubjectMembership" {
						continue
					}
					subjects, _ := model.ReflectiveElements(rel, "ownedRelatedElement")
					values, _ := model.ReflectiveElements(subjects[0], "ownedRelationship")
					expressions, _ := model.ReflectiveElements(values[0], "ownedRelatedElement")
					return expressions[0]
				}
			}
		}
		t.Fatalf("%s: no satisfy subject value", owner)
		return nil
	}
	plain := valueOf("observatory")
	referent, _ := model.ReflectiveElements(plain, "referent")
	if len(referent) != 1 || referent[0].Name != "telescope" {
		t.Errorf("by telescope: referent %v, want the part telescope", referent)
	}
	chained := valueOf("site")
	chainReferent, _ := model.ReflectiveElements(chained, "referent")
	if len(chainReferent) != 1 || chainReferent[0].Chain == nil && chainReferent[0].Expr == nil {
		t.Fatalf("by telescope.sub: referent %v, want its chain Feature", chainReferent)
	}
	links := model.ChainingFeatures(chainReferent[0])
	if len(links) != 2 || links[0].Name != "telescope" || links[1].Name != "sub" {
		t.Errorf("by telescope.sub: chainingFeature %v, want telescope, sub", links)
	}
}
