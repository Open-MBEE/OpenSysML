package repl

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// A paused %action resumes with its own evaluation budget: an %eval run between
// two steps spends its own, so a step that fit the budget before still fits.
func TestStepAfterEvalKeepsTheActionsOwnBudget(t *testing.T) {
	s := loadSource(t, choiceForkSource+`
package K {
	private import ScalarValues::*;
	attribute k : Integer = 3;
}
`)
	budgets := runtime.DefaultBudgets()
	budgets.MaxSteps = 30
	if err := s.SetBudgets(budgets); err != nil {
		t.Fatalf("SetBudgets: %v", err)
	}
	run(t, s, "%action tally")
	wants(t, run(t, s, "%step"), "✓ Step complete")
	wants(t, run(t, s, "%step"), "✓ Step complete")

	// An evaluation that spends most of a run's budget on its own.
	wants(t, run(t, s, "%eval K::k + K::k + K::k + K::k + K::k + K::k + K::k + K::k + K::k + K::k"), "= 30")

	got := run(t, s, "%step")
	wants(t, got, "✓ Step complete", "1 choice point")
	rejects(t, got, "step limit")
	wants(t, run(t, s, "%continue"), "✓ Action completed")
}
