package repl

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// TestSessionBudgetsDefault: a fresh session runs on the default bounds.
func TestSessionBudgetsDefault(t *testing.T) {
	if got := NewSession().Budgets(); got != runtime.DefaultBudgets() {
		t.Errorf("Budgets() = %+v, want %+v", got, runtime.DefaultBudgets())
	}
}

// TestSetBudgets: raised bounds are adopted and apply to the runtime context
// created afterwards; a non-positive bound is refused.
func TestSetBudgets(t *testing.T) {
	s := NewSession()
	if res := s.Submit("package P { part def Q; }"); len(res.Diagnostics) > 0 {
		t.Fatalf("submit reported diagnostics: %v", res.Diagnostics)
	}
	if _, err := s.getOrCreateRuntime(); err != nil {
		t.Fatalf("getOrCreateRuntime: %v", err)
	}

	s.instances["P::Q"] = &runtime.Instance{ID: 1}

	want := runtime.Budgets{MaxSteps: 4200, MaxActionSteps: 42, MaxStateEvents: 43, MaxDoSteps: 44, MaxElements: 45, MaxCalcDepth: 46, MaxSweepRuns: 47}
	if err := s.SetBudgets(want); err != nil {
		t.Fatalf("SetBudgets: %v", err)
	}
	if got := s.Budgets(); got != want {
		t.Errorf("Budgets() = %+v, want %+v", got, want)
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		t.Fatalf("getOrCreateRuntime: %v", err)
	}
	if got := ctx.Budgets(); got != want {
		t.Errorf("runtime context bounds = %+v, want %+v", got, want)
	}
	// Instances belonged to the discarded context, whose IDs the new one reuses.
	if len(s.instances) != 0 {
		t.Errorf("instances survived the new context: %v", s.instances)
	}

	for _, bad := range []runtime.Budgets{
		{MaxSteps: 0, MaxActionSteps: 1, MaxStateEvents: 1, MaxDoSteps: 1, MaxElements: 1, MaxCalcDepth: 1, MaxSweepRuns: 1},
		{MaxSteps: 1, MaxActionSteps: 1, MaxStateEvents: -1, MaxDoSteps: 1, MaxElements: 1, MaxCalcDepth: 1, MaxSweepRuns: 1},
		{MaxSteps: 1, MaxActionSteps: 1, MaxStateEvents: 1, MaxDoSteps: 1, MaxElements: -1, MaxCalcDepth: 1, MaxSweepRuns: 1},
		{MaxSteps: 1, MaxActionSteps: 1, MaxStateEvents: 1, MaxDoSteps: 1, MaxElements: 1, MaxCalcDepth: -1, MaxSweepRuns: 1},
		{MaxSteps: 1, MaxActionSteps: 1, MaxStateEvents: 1, MaxDoSteps: 1, MaxElements: 1, MaxCalcDepth: 1, MaxSweepRuns: -1},
		{},
	} {
		if err := s.SetBudgets(bad); err == nil {
			t.Errorf("SetBudgets(%+v) was accepted", bad)
		}
	}
	if got := s.Budgets(); got != want {
		t.Errorf("a refused set changed the bounds to %+v", got)
	}
}

// TestBudgetCommandShowsEveryBound: %budget reports the session's own bounds,
// each named with the variable that raises it.
func TestBudgetCommandShowsEveryBound(t *testing.T) {
	s := NewSession()
	budgets := runtime.DefaultBudgets()
	budgets.MaxElements = 4242
	if err := s.SetBudgets(budgets); err != nil {
		t.Fatalf("SetBudgets: %v", err)
	}
	wants(t, run(t, s, "%budget"),
		runtime.MaxStepsEnvVar,
		runtime.MaxActionStepsEnvVar,
		runtime.MaxStateEventsEnvVar,
		runtime.MaxDoStepsEnvVar,
		"4242",
		runtime.MaxElementsEnvVar,
		runtime.MaxCalcDepthEnvVar,
		runtime.MaxSweepRunsEnvVar)
}

// TestAdvanceIsBoundedBySessionBudgets: %advance drains a machine that never
// settles up to the session's own bounds, and says which one stopped it instead
// of looking like a machine that had settled.
func TestAdvanceIsBoundedBySessionBudgets(t *testing.T) {
	t.Run("event_budget", func(t *testing.T) {
		s := loadFixture(t, "testdata/state_spin.sysml")
		budgets := runtime.DefaultBudgets()
		budgets.MaxStateEvents = 7
		if err := s.SetBudgets(budgets); err != nil {
			t.Fatalf("SetBudgets: %v", err)
		}
		run(t, s, "%state Spin")
		wants(t, run(t, s, "%advance 1000"),
			"(7 event(s) processed)",
			"Stopped at the event budget (7 events; raise "+runtime.MaxStateEventsEnvVar,
			"All 7 event(s) were processed at simulation time 0.0 without advancing it")
	})

	// A finite same-time chain longer than the budget gets the one-instant note,
	// which does not claim the chain is endless.
	t.Run("event_budget_finite_chain", func(t *testing.T) {
		s := loadFixture(t, "testdata/state_chain.sysml")
		budgets := runtime.DefaultBudgets()
		budgets.MaxStateEvents = 2
		if err := s.SetBudgets(budgets); err != nil {
			t.Fatalf("SetBudgets: %v", err)
		}
		run(t, s, "%state Chain")
		wants(t, run(t, s, "%advance 1000"),
			"Stopped at the event budget (2 events; raise "+runtime.MaxStateEventsEnvVar,
			"All 2 event(s) were processed at simulation time 0.0 without advancing it")
	})

	// A chain that completes on the final budgeted event has settled: no
	// one-instant note, and the completion line still shows.
	t.Run("event_budget_completed_on_last_event", func(t *testing.T) {
		s := loadFixture(t, "testdata/state_chain.sysml")
		budgets := runtime.DefaultBudgets()
		budgets.MaxStateEvents = 3
		if err := s.SetBudgets(budgets); err != nil {
			t.Fatalf("SetBudgets: %v", err)
		}
		run(t, s, "%state Chain")
		got := run(t, s, "%advance 1000")
		wants(t, got, "State machine completed")
		rejects(t, got, "without advancing it")
	})

	// A machine whose loop is timed makes progress each cycle, so the budget
	// stop must not claim a larger budget cannot help.
	t.Run("event_budget_time_advancing", func(t *testing.T) {
		s := loadFixture(t, "testdata/state_tick.sysml")
		budgets := runtime.DefaultBudgets()
		budgets.MaxStateEvents = 7
		if err := s.SetBudgets(budgets); err != nil {
			t.Fatalf("SetBudgets: %v", err)
		}
		run(t, s, "%state Tick")
		got := run(t, s, "%advance 1000")
		wants(t, got,
			"Stopped at the event budget (7 events; raise "+runtime.MaxStateEventsEnvVar)
		rejects(t, got, "without advancing it")
	})

	t.Run("do_budget", func(t *testing.T) {
		s := loadFixture(t, "testdata/state_spin.sysml")
		budgets := runtime.DefaultBudgets()
		budgets.MaxDoSteps = 5
		if err := s.SetBudgets(budgets); err != nil {
			t.Fatalf("SetBudgets: %v", err)
		}
		run(t, s, "%state Spin")
		wants(t, run(t, s, "%advance 1000"),
			"Stopped at the do action budget (5 steps; raise "+runtime.MaxDoStepsEnvVar)
	})
}
