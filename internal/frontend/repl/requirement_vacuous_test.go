package repl

import "testing"

// assumptionOnlyModel states a requirement that assumes but requires nothing of
// an object whose value fails the assumption.
const assumptionOnlyModel = `
package Vac {
    private import ScalarValues::Integer;
    part def Rig {
        attribute level : Integer = 0;
        requirement onlyAssumed { assume constraint { level > 1 } }
    }
    part rig : Rig;
}
`

// TestAssumptionOnlyRequirementAgreesAcrossEngines: a requirement's assumptions
// are the antecedent of its claim, so one requiring nothing holds vacuously
// however they are met — as `%requirement` observes, `%validate` judges and the
// solver answers under `%check`.
func TestAssumptionOnlyRequirementAgreesAcrossEngines(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, assumptionOnlyModel)
	wants(t, run(t, s, "%requirement Vac::Rig::onlyAssumed"),
		"✓ Requirement Vac::Rig::onlyAssumed satisfied",
		"vacuous: requirement onlyAssumed states no required constraint, so it holds vacuously")
	wants(t, run(t, s, "%check Vac::Rig::onlyAssumed"), "✓ Requirement onlyAssumed is satisfiable")
	wants(t, run(t, s, "%solve Vac::Rig::onlyAssumed"), "✓ Requirement onlyAssumed has values satisfying it")
	run(t, s, "%instantiate Vac::rig")
	wants(t, run(t, s, "%validate rig"),
		"✓ requirement onlyAssumed holds (on Vac::rig ID: 1)",
		"✓ Vac::rig is valid: its 1 assertion holds")
}
