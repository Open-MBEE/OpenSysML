package runtime

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// stateStarterOf starts the machine alone on the clock, up to the horizon.
func stateStarterOf(sym *symbols.Symbol, horizon Horizon) Starter {
	return func(ctx *Context) (*Invocation, error) {
		exec, err := ctx.CreateStateExecutor(sym)
		if err != nil {
			return nil, err
		}
		return &Invocation{States: []*StateExecutor{exec}, Horizon: horizon}, nil
	}
}

// tickerModel is a machine re-arming a timer every 2 seconds, counting its ticks.
func tickerModel(t *testing.T) *exploreModel {
	t.Helper()
	return parseLibraryModel(t, `
		package test {
			private import SI::*;
			private import ScalarValues::*;
			state Ticker {
				attribute ticks : Integer = 0;
				entry; then armed;
				state armed;
				transition first armed accept after 2 [s] do assign ticks := ticks + 1 then armed;
			}
		}
	`)
}

// A machine re-arming a timer has no last state: without a horizon the search
// runs to the events budget and says so; with one it is exhaustive up to the
// horizon, its final the configuration the machine rests in there.
func TestCheckHorizonBoundsARearmingTimer(t *testing.T) {
	m := tickerModel(t)
	sym := m.state(t, "Ticker")
	fresh := func() (*Context, error) {
		ctx, err := m.fresh()
		if err != nil {
			return nil, err
		}
		budgets := ctx.Budgets()
		budgets.MaxStateEvents = 5
		return ctx, ctx.SetBudgets(budgets)
	}

	unbounded, err := Check(context.Background(), fresh, stateStarterOf(sym, Horizon{}), CheckBudget{}, unreduced(), nil)
	if err != nil {
		t.Fatalf("check without a horizon: %v", err)
	}
	if unbounded.Verdict != CheckWithinBounds || !slices.Equal(unbounded.BoundsHit, []string{BoundEvents}) {
		t.Fatalf("without a horizon: %s, want within bounds hitting %s alone", unbounded.Status(), BoundEvents)
	}
	if unbounded.MaxDepth != 5 || len(unbounded.Finals) != 0 {
		t.Fatalf("without a horizon: depth %d, %d finals, want the 5 dispatches the budget allows and no final", unbounded.MaxDepth, len(unbounded.Finals))
	}

	bounded, err := Check(context.Background(), fresh, stateStarterOf(sym, HorizonAt(5)), CheckBudget{}, unreduced(), nil)
	if err != nil {
		t.Fatalf("check to t=5: %v", err)
	}
	if bounded.Verdict != CheckExhaustive || len(bounded.BoundsHit) != 0 {
		t.Fatalf("to t=5: %s, want exhaustive", bounded.Status())
	}
	if !strings.HasPrefix(bounded.Status(), "no violation, exhaustive up to t=5.0 (") {
		t.Fatalf("to t=5: status %q, want the horizon told", bounded.Status())
	}
	if len(bounded.Finals) != 1 || bounded.Finals[0].Values["ticks"] != "2" || bounded.Finals[0].Values["finalState"] != "armed" {
		t.Fatalf("to t=5: finals %+v, want one at armed with two ticks", bounded.Finals)
	}
	if bounded.MaxDepth != 2 {
		t.Fatalf("to t=5: depth %d, want the two dispatches within the horizon", bounded.MaxDepth)
	}

	// A horizon at t=0 is not none: the machine rests armed with its timer ahead.
	atStart, err := Check(context.Background(), fresh, stateStarterOf(sym, HorizonAt(0)), CheckBudget{}, unreduced(), nil)
	if err != nil {
		t.Fatalf("check to t=0: %v", err)
	}
	if !strings.HasPrefix(atStart.Status(), "no violation, exhaustive up to t=0.0 (") ||
		len(atStart.Finals) != 1 || atStart.Finals[0].Values["ticks"] != "0" || atStart.Finals[0].Values["finalState"] != "armed" {
		t.Fatalf("to t=0: %s, finals %+v; want exhaustive at armed with no tick", atStart.Status(), atStart.Finals)
	}
}

// A property is evaluated at the horizon as at every state: one false only past
// it holds, one false at it is violated with the schedule reaching it.
func TestCheckHorizonEvaluatesPropertiesUpToIt(t *testing.T) {
	m := tickerModel(t)
	sym := m.state(t, "Ticker")
	ticksUnder := func(limit int64) CheckProperty {
		return CheckProperty{Name: "ticks under limit", Holds: func(_ *Context, inv *Invocation) (bool, error) {
			ticks := inv.States[0].StateData()["ticks"]
			return ticks.Kind == ValConst && ticks.Const.Int < limit, nil
		}}
	}

	holds, err := Check(context.Background(), m.fresh, stateStarterOf(sym, HorizonAt(5)), CheckBudget{}, unreduced(), []CheckProperty{ticksUnder(3)})
	if err != nil {
		t.Fatalf("check ticks < 3 to t=5: %v", err)
	}
	if holds.Verdict != CheckExhaustive {
		t.Fatalf("ticks < 3 to t=5: %s, want exhaustive", holds.Status())
	}

	violated, err := Check(context.Background(), m.fresh, stateStarterOf(sym, HorizonAt(5)), CheckBudget{}, unreduced(), []CheckProperty{ticksUnder(2)})
	if err != nil {
		t.Fatalf("check ticks < 2 to t=5: %v", err)
	}
	if violated.Verdict != CheckViolation || len(violated.Violations) != 1 {
		t.Fatalf("ticks < 2 to t=5: %s, want the one violation", violated.Status())
	}
	v := violated.Violations[0]
	if v.Kind != ViolationProperty || v.Name != "ticks under limit" || v.Depth != 2 {
		t.Fatalf("violation %+v, want the property false after the second dispatch", v)
	}
}

// An action waiting on the clock past the horizon is final there, not deadlocked;
// without a horizon its wait comes due and the action completes.
func TestCheckHorizonLeavesAnActionWaitingPastIt(t *testing.T) {
	m := parseLibraryModel(t, `
		package test {
			private import SI::*;
			private import ScalarValues::*;
			action def Sleeper {
				attribute out x : Integer = 0;
				first start;
				then accept after 10 [s];
				then assign x := 1;
				then done;
			}
		}
	`)
	sym := m.action(t, "Sleeper")
	startTo := func(horizon Horizon) Starter {
		return func(ctx *Context) (*Invocation, error) {
			exec, err := ctx.CreateActionExecutor(sym)
			if err != nil {
				return nil, err
			}
			return &Invocation{Actions: []*ActionExecutor{exec}, Horizon: horizon}, nil
		}
	}

	early, err := Check(context.Background(), m.fresh, startTo(HorizonAt(5)), CheckBudget{}, unreduced(), nil)
	if err != nil {
		t.Fatalf("check to t=5: %v", err)
	}
	if early.Verdict != CheckExhaustive || len(early.Violations) != 0 {
		t.Fatalf("to t=5: %s, want exhaustive with the action still waiting", early.Status())
	}
	if len(early.Finals) != 1 || early.Finals[0].Values["x"] != "0" {
		t.Fatalf("to t=5: finals %+v, want x still 0", early.Finals)
	}

	late, err := Check(context.Background(), m.fresh, startTo(Horizon{}), CheckBudget{}, unreduced(), nil)
	if err != nil {
		t.Fatalf("check without a horizon: %v", err)
	}
	if late.Verdict != CheckExhaustive || len(late.Finals) != 1 || late.Finals[0].Values["x"] != "1" {
		t.Fatalf("without a horizon: %s, finals %+v, want x = 1", late.Status(), late.Finals)
	}
}

// A machine's failure modes are violations on the schedule reaching them, each
// with its witness, as an action's are; a machine that cannot initialize fails
// the start, a violation before any move.
func TestCheckReportsStateFailuresAsViolations(t *testing.T) {
	cases := []struct {
		name, src string
		err       error
		depth     int
	}{
		{"no initial state", `package test {
			state Machine {
				state idle;
				state busy;
				transition first idle then busy;
			}
		}`, ErrNoInitialState, 0},
		{"choice without a branch", `package test {
			private import ScalarValues::*;
			state Machine {
				attribute x : Integer = 0;
				entry; then init;
				state init;
				state busy;
				choice pick;
				state seen;
				succession first init then busy;
				transition first busy do assign x := 2 then pick;
				transition first pick if x == 1 then seen;
			}
		}`, ErrChoiceWithoutBranch, 2},
		{"effect reading an unknown feature", `package test {
			private import ScalarValues::*;
			state Machine {
				attribute x : Integer = 0;
				entry; then init;
				state init;
				state active;
				transition first init do assign x := missingName + 1 then active;
			}
		}`, ErrUnresolvedReference, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := parseExploreModel(t, c.src)
			start := stateStarterOf(m.state(t, "Machine"), Horizon{})
			report, err := Check(context.Background(), m.fresh, start, CheckBudget{}, reduced(), nil)
			if err != nil {
				t.Fatalf("check: %v", err)
			}
			if report.Verdict != CheckViolation || len(report.Violations) != 1 {
				t.Fatalf("verdict %s, violations %v; want one violation", report.Status(), report.Violations)
			}
			v := report.Violations[0]
			if v.Kind != ViolationFailure || !errors.Is(v.Err, c.err) || v.Depth != c.depth {
				t.Fatalf("violation %s (%v) at depth %d, want a failure wrapping %v at depth %d", v.Kind, v.Err, v.Depth, c.err, c.depth)
			}
			if v.Witness.Fails != v.Err.Error() {
				t.Fatalf("the witness claims %q, want the violation's %v", v.Witness.Fails, v.Err)
			}
			r, err := Replay(context.Background(), m.fresh, start, v.Witness, nil)
			if err != nil {
				t.Fatalf("replay: %v", err)
			}
			if !errors.Is(r.Err, c.err) {
				t.Fatalf("the replay ends with %v, want %v", r.Err, c.err)
			}
		})
	}
}

// A caller's deadline stops a search with no end of its own as any other,
// reporting what it searched by then.
func TestCheckStopsARearmingTimerOnTheCallerDeadline(t *testing.T) {
	m := tickerModel(t)
	sym := m.state(t, "Ticker")
	stop, cancel := context.WithCancel(context.Background())
	visits := 0
	deadline := CheckProperty{Name: "deadline", Holds: func(*Context, *Invocation) (bool, error) {
		if visits++; visits == 4 {
			cancel()
		}
		return true, nil
	}}
	_, err := Check(stop, m.fresh, stateStarterOf(sym, Horizon{}), CheckBudget{}, unreduced(), []CheckProperty{deadline})
	var stopped *CheckStopped
	if !errors.As(err, &stopped) || !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want CheckStopped wrapping context.Canceled", err)
	}
	if stopped.Moves < 3 || stopped.Moves > 4 {
		t.Fatalf("stopped after %d moves, want the ones made by the fourth state", stopped.Moves)
	}
}
