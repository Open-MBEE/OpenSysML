package runtime

import (
	"context"
	"errors"
	"slices"
	"testing"
)

// busyDoModel spins its do flow forever while Stop waits in the pool: under one-move
// scheduling the dispatch is drawn against every move of the loop.
const busyDoModel = `package test {
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
}`

// TestRuntimeRobustnessDoStepTokenGrain: a do flow drawn one move at a time against a
// dispatch ends at the dispatch or a named budget; a body parked at an accept offers no draw.
func TestRuntimeRobustnessDoStepTokenGrain(t *testing.T) {
	t.Run("busy_do_flow_against_a_due_dispatch_ends_at_the_dispatch_or_the_budget", testBusyDoFlowAgainstADueDispatch)
	t.Run("check_of_a_busy_do_flow_against_a_due_dispatch_names_its_bound", testCheckOfABusyDoFlowNamesItsBound)
	t.Run("step_order_naming_a_do_body_parked_at_an_accept", testStepOrderNamingADoBodyParkedAtAnAccept)
}

// testBusyDoFlowAgainstADueDispatch: every explored run of an endless do flow with a
// dispatch due takes the dispatch after some moves or hits the do-step budget, a typed error.
func testBusyDoFlowAgainstADueDispatch(t *testing.T) {
	m := parseExploreModel(t, busyDoModel)
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
		exec.SendSignal("Stop", nil)
		err = exec.RunToCompletion()
		return exec.Outcome(), err
	}
	x, err := Explore(context.Background(), mustPolicy(t, "explore"), m.fresh, run)
	if err != nil {
		t.Fatalf("explore: %v", err)
	}
	if !x.Complete() || len(x.Outcomes) < 2 {
		t.Fatalf("exploration %s with %d outcomes, want complete with dispatched and budgeted runs", x.Status(), len(x.Outcomes))
	}
	var dispatched, budgeted int
	for _, o := range x.Outcomes {
		switch {
		case errors.Is(o.Outcome.Err, ErrDoStepLimitExceeded):
			budgeted++
		case o.Outcome.Err == nil && o.Outcome.FinalState == "idle":
			dispatched++
		default:
			t.Errorf("outcome %s: error %v, want the dispatch taken or the do-step budget", o.Outcome, o.Outcome.Err)
		}
	}
	if dispatched == 0 || budgeted == 0 {
		t.Errorf("%d dispatched, %d budgeted runs, want both", dispatched, budgeted)
	}
}

// testCheckOfABusyDoFlowNamesItsBound: the schedule that keeps moving the do flow never
// dispatches, so the check runs into an executor budget and names it, not exhaustive.
func testCheckOfABusyDoFlowNamesItsBound(t *testing.T) {
	m := parseExploreModel(t, busyDoModel)
	sym := m.state(t, "Machine")
	start := func(ctx *Context) (*Invocation, error) {
		budgets := ctx.Budgets()
		budgets.MaxSteps = 300
		if err := ctx.SetBudgets(budgets); err != nil {
			return nil, err
		}
		exec, err := ctx.CreateStateExecutor(sym)
		if err != nil {
			return nil, err
		}
		exec.SendSignal("Stop", nil)
		return &Invocation{States: []*StateExecutor{exec}}, nil
	}
	report, err := Check(context.Background(), m.fresh, start, CheckBudget{}, unreduced(), nil)
	if err != nil {
		t.Fatal(err)
	}
	bounded := slices.ContainsFunc(report.BoundsHit, func(b string) bool { return slices.Contains(ExecutorBounds, b) })
	if report.Verdict == CheckExhaustive || !bounded {
		t.Fatalf("check: %s, want bounded by an executor budget", report.Status())
	}
	if !slices.ContainsFunc(report.Finals, func(f CheckFinal) bool { return f.Outcome == "finalState idle; visits top, idle; n = 0" }) {
		t.Fatalf("finals %v, want the dispatch before any move of the do flow among them", finalOutcomes(report))
	}
}

// testStepOrderNamingADoBodyParkedAtAnAccept: a do body parked at an accept of a signal not
// in the pool is not due, so a second step-order line finds no draw: refused as unfollowed.
func testStepOrderNamingADoBodyParkedAtAnAccept(t *testing.T) {
	m := parseExploreModel(t, `package test {
		private import ScalarValues::*;
		attribute def Stop;
		attribute def Go;
		state def Machine {
			attribute log : String = "";
			entry; then top;
			state top {
				do action wait { first start; then action w accept Go; then action mark assign log := log + "go "; then done; }
			}
			transition first top accept Stop do assign log := log + "stop " then idle;
			state idle;
		}
	}`)
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
	if log := FormatValue(exec.StateData()["log"]); log != `"stop "` {
		t.Errorf("log is %s, want the dispatch alone after the move that parked the body", log)
	}
	err = ctx.Unfollowed()
	var refused *ReplayError
	if !errors.As(err, &refused) || refused.Move != 2 {
		t.Fatalf("error %T %v, want the line refused as unfollowed", err, err)
	}
}
