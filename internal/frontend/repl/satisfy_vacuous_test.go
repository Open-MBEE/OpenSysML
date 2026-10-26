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
