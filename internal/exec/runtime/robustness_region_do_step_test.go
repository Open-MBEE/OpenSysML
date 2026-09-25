package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// doStepModel has Stop in the pool as top is entered, whose do behavior logs
// once: the do step and the dispatch are due together at t=0.
const doStepModel = `package test {
	private import ScalarValues::*;
	attribute def Stop;
	state def Machine {
		attribute log : String = "";
		entry; then top;
		state top {
			do { assign log := log + "did "; }
		}
		transition first top accept Stop do assign log := log + "stop " then idle;
		state idle;
	}
}`

// TestRuntimeRobustnessRegionDoStep: a step-order line naming a move the unit does not
// offer is a typed refusal leaving the data as the move found it; a do behavior that
// never ends is stopped by the do-step budget under one-move scheduling as under a round.
func TestRuntimeRobustnessRegionDoStep(t *testing.T) {
	t.Run("step_order_naming_a_state_with_no_due_do_step", testStepOrderNamingAStateWithNoDueDoStep)
	t.Run("step_order_naming_a_dispatch_not_at_the_head", testStepOrderNamingADispatchNotAtTheHead)
	t.Run("step_order_at_a_unit_offering_no_draw", testStepOrderAtAUnitOfferingNoDraw)
	t.Run("endless_do_behavior_under_explore_hits_the_do_step_budget", testEndlessDoBehaviorUnderExploreHitsTheDoStepBudget)
}

// refusedDoStepReplay drives doStepModel under the witness with Stop in the pool and
// returns the refusal, with the log the refusal left behind.
func refusedDoStepReplay(t *testing.T, lines string) (*ReplayError, string) {
	t.Helper()
	m := parseExploreModel(t, doStepModel)
	sym := m.state(t, "Machine")
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	witness, err := ParseChoices(lines)
	if err != nil {
		t.Fatal(err)
	}
	mustSchedule(t, ctx, ReplayPolicy(witness))
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		t.Fatal(err)
	}
	exec.SendSignal("Stop", nil)
	err = exec.RunToCompletion()
	var refused *ReplayError
	if !errors.As(err, &refused) || !errors.Is(err, ErrReplayRefused) {
		t.Fatalf("error %T %v, want a ReplayError", err, err)
	}
	return refused, FormatValue(exec.StateData()["log"])
}

// testStepOrderNamingAStateWithNoDueDoStep: a line naming a do step of a state with no
// due do behavior is refused at the draw, before the do behavior or the dispatch acts.
func testStepOrderNamingAStateWithNoDueDoStep(t *testing.T) {
	refused, log := refusedDoStepReplay(t, "at t=0.0: do idle first of do idle, dispatch accept Stop\n")
	if refused.Move != 1 || !strings.Contains(refused.Error(), "do idle is not enabled (enabled: do top, dispatch accept Stop)") {
		t.Errorf("refused %v, want move 1 naming do idle as not enabled among do top, dispatch accept Stop", refused)
	}
	if log != `""` {
		t.Errorf("log is %s after the refusal, want empty: neither the do step nor the dispatch may run on a refused draw", log)
	}
}

// testStepOrderNamingADispatchNotAtTheHead: a line naming a dispatch of an event not at
// the head of the pool is refused at the draw, the pool untouched.
func testStepOrderNamingADispatchNotAtTheHead(t *testing.T) {
	refused, log := refusedDoStepReplay(t, "at t=0.0: dispatch accept Go first of do top, dispatch accept Go\n")
	if refused.Move != 1 || !strings.Contains(refused.Error(), "dispatch accept Go is not enabled (enabled: do top, dispatch accept Stop)") {
		t.Errorf("refused %v, want move 1 naming dispatch accept Go as not enabled", refused)
	}
	if log != `""` {
		t.Errorf("log is %s after the refusal, want empty", log)
	}
}

// testStepOrderAtAUnitOfferingNoDraw: once the do step is taken the dispatch is owed and
// alone, so a second step-order line finds no draw and is refused as an unfollowed move
// after the run ends where the dispatch left it.
func testStepOrderAtAUnitOfferingNoDraw(t *testing.T) {
	m := parseExploreModel(t, doStepModel)
	sym := m.state(t, "Machine")
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	witness, err := ParseChoices("at t=0.0: do top first of do top, dispatch accept Stop\nat t=0.0: do top first of do top, dispatch accept Stop\n")
	if err != nil {
		t.Fatal(err)
	}
	mustSchedule(t, ctx, ReplayPolicy(witness))
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		t.Fatal(err)
	}
	exec.SendSignal("Stop", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if log := FormatValue(exec.StateData()["log"]); log != `"did stop "` {
		t.Errorf("log is %s, want the do step then the dispatch", log)
	}
	err = ctx.Unfollowed()
	var refused *ReplayError
	if !errors.As(err, &refused) || refused.Move != 2 {
		t.Fatalf("error %T %v, want the second line refused as unfollowed", err, err)
	}
}

// testEndlessDoBehaviorUnderExploreHitsTheDoStepBudget: a do behavior looping forever is
// stopped by the do-step budget on every explored run, a typed error, no hang.
func testEndlessDoBehaviorUnderExploreHitsTheDoStepBudget(t *testing.T) {
	m := parseExploreModel(t, `package test {
		private import ScalarValues::*;
		attribute def Stop;
		state def Machine {
			attribute n : Integer = 0;
			entry; then top;
			state top {
				do action spin { first start; then merge again; then action count assign n := n + 1; then again; }
			}
			transition first top accept Stop then idle;
			state idle;
		}
	}`)
	sym := m.state(t, "Machine")
	run := func(ctx *Context) (Outcome, error) {
		budgets := ctx.Budgets()
		budgets.MaxDoSteps = 8
		if err := ctx.SetBudgets(budgets); err != nil {
			return Outcome{}, err
		}
		exec, err := ctx.CreateStateExecutor(sym)
		if err != nil {
			return Outcome{}, err
		}
		return exec.Outcome(), exec.RunToCompletion()
	}
	x, err := Explore(context.Background(), mustPolicy(t, "explore"), m.fresh, run)
	if err != nil {
		t.Fatalf("explore: %v", err)
	}
	if !x.Complete() || len(x.Outcomes) == 0 {
		t.Fatalf("exploration %s with %d outcomes, want complete", x.Status(), len(x.Outcomes))
	}
	for _, o := range x.Outcomes {
		if !errors.Is(o.Outcome.Err, ErrDoStepLimitExceeded) {
			t.Errorf("outcome %s: error %v, want the do-step budget", o.Outcome, o.Outcome.Err)
		}
	}
}
