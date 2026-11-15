package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A constraint body's closing bare condition is its result, owned through a ResultExpressionMembership
// after the parameters, as the conversion writes it and the pilot exports it (SysML.xtext ownedResultExpression).
func TestConstraintBodyOwnsItsResultExpressionMembership(t *testing.T) {
	const src = `package P {
    constraint def MassBudget { in total : Real; total <= 30.0 }
    part def Subsystem { attribute mass : Real; }
    requirement def MassLimit { subject sub : Subsystem; require constraint { sub.mass <= 20.0 } }
}`
	for _, fixture := range reflectiveFixtures(t, "constraint-result.sysml", source.KindSysML, src) {
		model := fixture.model
		budget := nestedSym(t, fixture.root, "P::MassBudget")
		if budget.Recorded() {
			// A recorded symbol keeps no body expression.
			continue
		}
		if got, want := ownedMemberKinds(model, budget), "FeatureMembership:ReferenceUsage ResultExpressionMembership:OperatorExpression"; got != want {
			t.Errorf("MassBudget ownedRelationship = %s, want %s", got, want)
		}
		limit := nestedSym(t, fixture.root, "P::MassLimit")
		members := ownedMembers(model, limit)
		if len(members) != 2 {
			t.Fatalf("MassLimit owns %d members, want 2", len(members))
		}
		if got, want := ownedMemberKinds(model, members[1]), "ResultExpressionMembership:OperatorExpression"; got != want {
			t.Errorf("the require constraint's ownedRelationship = %s, want %s", got, want)
		}
	}
}
