package runtime

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// A start that fails after entering a state is a violation on every schedule
// from it; the coverage still holds the states the failed start entered.
func TestCheckCoverageHoldsTheStatesAFailedStartEntered(t *testing.T) {
	m := parseExploreModel(t, `package test {
		private import ScalarValues::*;
		action def Adder { in a : Integer[1]; in b : Integer[1]; out sum : Integer; first step; action step { assign sum := a + b; } }
		action adder : Adder;
		state run {
			entry; then s0;
			state s0 {
				entry perform adder;
			}
			state fin;
			transition first s0 then fin;
		}
	}`)
	report, err := Check(context.Background(), m.fresh, invocationOf(nil, []*symbols.Symbol{m.state(t, "run")}), CheckBudget{}, reduced(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != CheckViolation || len(report.Violations) != 1 || report.Violations[0].Kind != ViolationFailure {
		t.Fatalf("verdict %s, violations %+v; want one failure", report.Status(), report.Violations)
	}
	var entered []string
	for _, entry := range report.Coverage.States {
		entered = append(entered, fmt.Sprintf("%s=%d", entry.Declaration, entry.Runs))
	}
	if !slices.Contains(entered, "test::run::s0=1") {
		t.Errorf("states entered %q lack s0 entered by the failed schedule", entered)
	}
}
