package repl

import (
	"strings"
	"testing"
)

// vacuousViewpointModel frames a concern in a viewpoint that requires nothing, and
// a view that satisfies the viewpoint.
const vacuousViewpointModel = `
package Reading {
	part def Mission;
	part mission : Mission;
	concern def Traceability {
		subject mission : Mission;
	}
	concern traceability : Traceability;
	viewpoint def Overview {
		frame traceability;
	}
	viewpoint reading : Overview;
	view tree {
		expose mission;
		satisfy reading;
	}
	requirement def Prose { doc /* stated in prose only */ }
	requirement prose : Prose;
	part context {
		assert not satisfy prose by mission;
	}
}
`

// TestSatisfyVacuousViewpointHolds pins the text of a vacuous verdict: a `satisfy`
// of a viewpoint with no required constraint holds, its standing says why, and
// the denial of a condition-less requirement fails with the same reason.
func TestSatisfyVacuousViewpointHolds(t *testing.T) {
	s := loadSource(t, vacuousViewpointModel)
	out := run(t, s, "%satisfy")
	wantsInOrder(t, out,
		"✓ satisfy reading holds (on Reading::tree ID: ",
		"  standing: holds (observed: 1 run under reverse; vacuous: viewpoint reading states no required constraint, so it holds vacuously)",
		"✗ not satisfy prose by mission fails (on Reading::mission ID: ",
		"  Required condition evaluated to false: requirement prose states no required constraint, so it holds vacuously",
		"  standing: violated (witnessed: 1 run under reverse)",
	)
	if strings.Contains(out, "could not be evaluated") || strings.Contains(out, "no value") {
		t.Errorf("a vacuous requirement was reported unevaluable:\n%s", out)
	}
	wants(t, run(t, s, "%requirement Reading::prose"),
		"✓ Requirement Reading::prose satisfied",
		"vacuous: requirement prose states no required constraint, so it holds vacuously")
}

// failedAssumptionModel is the issue's reproducer: a requirement whose assumption
// `off` fails and whose required constraint `off` and `on` both fail.
const failedAssumptionModel = `
package Repro {
	private import ScalarValues::*;
	part def Thing { attribute enabled : Boolean; attribute level : Integer; }
	requirement def LevelOk {
		subject t : Thing;
		assume constraint { t.enabled }
		require constraint { t.level == 1 }
	}
	part off : Thing { attribute :>> enabled = false; attribute :>> level = 5; }
	part on : Thing { attribute :>> enabled = true; attribute :>> level = 5; }
	requirement levelOk : LevelOk;
	satisfy levelOk by off;
	satisfy levelOk by on;
	assert not satisfy levelOk by off;
	requirement offReq : LevelOk { subject t = off; }
}
`

// TestSatisfyFailedAssumptionHoldsVacuously pins the verdict text of a failed assumption: `off`
// satisfies the requirement naming it, `on` fails the required constraint, denying `off` fails.
func TestSatisfyFailedAssumptionHoldsVacuously(t *testing.T) {
	s := loadSource(t, failedAssumptionModel)
	wantsInOrder(t, run(t, s, "%satisfy"),
		"✓ satisfy levelOk by off holds (on Repro::off ID: ",
		"  standing: holds (observed: 1 run under reverse; vacuous: assumption t.enabled of requirement levelOk did not hold, so it holds vacuously)",
		"✗ satisfy levelOk by on fails (on Repro::on ID: ",
		"  Required condition evaluated to false: t.level == 1",
		"✗ not satisfy levelOk by off fails (on Repro::off ID: ",
		"  Required condition evaluated to false: assumption t.enabled of requirement levelOk did not hold, so it holds vacuously",
	)
	wants(t, run(t, s, "%requirement Repro::offReq"),
		"✓ Requirement Repro::offReq satisfied",
		"vacuous: assumption t.enabled of requirement offReq did not hold, so it holds vacuously")
}
