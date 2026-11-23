package queryexec

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// TestLoopAndAssignmentExpressionsHaveTheirOwnElementIDs: a loop's collection and conditions and an
// assignment's target and value are identified, with their memberships, as the conversion mints them.
func TestLoopAndAssignmentExpressionsHaveTheirOwnElementIDs(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
	package P {
		part def PD { attribute w; }
		part p : PD;
		action def A {
			attribute x = 0;
			action f for i in 1..3 { action a1; }
			action w while x < 3 { action a2; } until x > 5;
			action l loop { action a3; } until x > 1;
			action s assign p.w := 1;
			for j in 1..2 { action a4; }
			while x < 2 { action a5; }
			assign p.w := 2;
		}
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
	written := apiJSONClasses(t, fixture)
	for id, path := range ids {
		if class := path[strings.LastIndex(path, "/")+1:]; written[id] != class {
			t.Errorf("elementId %q of %s: the conversion writes %q there", id, path, written[id])
		}
	}
	const action = "/Package/OwningMembership/ActionDefinition/FeatureMembership"
	const forLoop, whileLoop, assignment = action + "/ForLoopActionUsage", action + "/WhileLoopActionUsage", action + "/AssignmentActionUsage"
	for id, want := range map[string]string{
		"Observatory__P__A__f_pcollection":            forLoop + "/OwningMembership/OperatorExpression",
		"Observatory__P__A__f_pcollection_om":         forLoop + "/OwningMembership",
		"Observatory__P__A__f_pcollection_pa0":        forLoop + "/OwningMembership/OperatorExpression/ParameterMembership/Feature/FeatureValue/LiteralInteger",
		"Observatory__P__A__f___400__a1_om":           forLoop + "/FeatureMembership",
		"Observatory__P__A__w_pwhileCondition":        whileLoop + "/OwningMembership/OperatorExpression",
		"Observatory__P__A__w_pwhileCondition_om":     whileLoop + "/OwningMembership",
		"Observatory__P__A__w_puntilCondition":        whileLoop + "/OwningMembership/OperatorExpression",
		"Observatory__P__A__w_puntilCondition_om":     whileLoop + "/OwningMembership",
		"Observatory__P__A__w___400__a2_om":           whileLoop + "/FeatureMembership",
		"Observatory__P__A__l_puntilCondition":        whileLoop + "/OwningMembership/OperatorExpression",
		"Observatory__P__A__l_puntilCondition_om":     whileLoop + "/OwningMembership",
		"Observatory__P__A__l___400__a3_om":           whileLoop + "/FeatureMembership",
		"Observatory__P__A__s_ptarget":                assignment + "/OwningMembership/FeatureChainExpression",
		"Observatory__P__A__s_ptarget_om":             assignment + "/OwningMembership",
		"Observatory__P__A__s_ptarget_ptargetFeature": assignment + "/OwningMembership/FeatureChainExpression/Membership",
		"Observatory__P__A__s_pvalue":                 assignment + "/FeatureValue/LiteralInteger",
		"Observatory__P__A__s_pvalue_om":              assignment + "/FeatureValue",
		"Observatory__P__A___405_pcollection":         forLoop + "/OwningMembership/OperatorExpression",
		"Observatory__P__A___405__a4_om":              forLoop + "/FeatureMembership",
		"Observatory__P__A___406_pwhileCondition":     whileLoop + "/OwningMembership/OperatorExpression",
		"Observatory__P__A___406_pwhileCondition_om":  whileLoop + "/OwningMembership",
		"Observatory__P__A___407_ptarget":             assignment + "/OwningMembership/FeatureChainExpression",
		"Observatory__P__A___407_pvalue":              assignment + "/FeatureValue/LiteralInteger",
		"Observatory__P__A___407_pvalue_om":           assignment + "/FeatureValue",
	} {
		if got := ids[id]; got != want {
			t.Errorf("elementId %q identifies %q, want %s", id, got, want)
		}
	}
}
