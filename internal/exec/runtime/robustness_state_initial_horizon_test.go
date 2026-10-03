package runtime

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

// TestRuntimeRobustnessStateInitialHorizon exercises a check of machines stopped
// at their initial transition: the search takes the entry moves alone, holds the
// clock, scopes its outcome when work is left, and refuses an action beside it.
func TestRuntimeRobustnessStateInitialHorizon(t *testing.T) {
	t.Run("pending_do_body_is_scoped", testInitialHorizonScopesPendingDo)
	t.Run("rearming_timer_stops_at_entry", testInitialHorizonStopsARearmingTimer)
	t.Run("nothing_pending_is_not_scoped", testInitialHorizonNothingPending)
	t.Run("region_entry_orders_still_searched", testInitialHorizonSearchesRegionEntry)
	t.Run("action_beside_it_is_refused", testInitialHorizonRefusesAnAction)
	t.Run("advanced_horizon_runs_the_do_body", testAdvancedHorizonRunsTheDoBody)
}

const initialHorizonModel = `package test {
	private import ScalarValues::*;
	private import SI::*;
	item def Go;
	state def Waits {
		attribute c : Integer := 0;
		entry; then s;
		state s { do action { assign c := 1; then accept after 2 [SI::s]; then assign c := 2; } }
	}
	state def Idle {
		attribute c : Integer := 0;
		entry; then a;
		state a;
		transition first a accept Go then b;
		state b;
	}
	state def Pair {
		attribute log : String = "";
		entry; then work;
		state work parallel {
			state left {
				entry { assign log := log + "left "; } then l;
				state l { entry { assign log := log + "l "; } }
			}
			state right {
				entry { assign log := log + "right "; } then r;
				state r { entry { assign log := log + "r "; } }
			}
		}
	}
	action noop { attribute x : Integer := 1; }
}`

// checkStopped checks the machine under the horizon, failing on any error.
func checkStopped(t *testing.T, m *exploreModel, machine string, horizon Horizon) *CheckReport {
	t.Helper()
	report, err := Check(context.Background(), m.fresh, stateStarterOf(m.state(t, machine), horizon), CheckBudget{}, unreduced(), nil)
	if err != nil {
		t.Fatalf("check %s: %v", machine, err)
	}
	return report
}

// A do body the initial transition started is left unrun: c keeps its initial
// value, no move is searched, and the report says where it was observed.
func testInitialHorizonScopesPendingDo(t *testing.T) {
	report := checkStopped(t, parseLibraryModel(t, initialHorizonModel), "Waits", HorizonInitial())
	if report.Verdict != CheckExhaustive || report.Moves != 0 {
		t.Fatalf("verdict %v after %d moves, want a clean search of no move", report.Verdict, report.Moves)
	}
	if got := finalOutcomes(report); len(got) != 1 || !strings.Contains(got[0], "c = 0") {
		t.Fatalf("finals = %v, want c = 0 alone", got)
	}
	if !slices.Equal(report.Scope, []ObservationReason{ReasonInitialTransitionOnly}) {
		t.Fatalf("scope = %v, want the initial transition only", report.Scope)
	}
}

// A timer re-arming itself never brings the search to its events budget: the
// clock holds at the initial transition.
func testInitialHorizonStopsARearmingTimer(t *testing.T) {
	report := checkStopped(t, tickerModel(t), "Ticker", HorizonInitial())
	if report.Verdict != CheckExhaustive || len(report.BoundsHit) > 0 {
		t.Fatalf("verdict %v, bounds %v, want a clean search within every bound", report.Verdict, report.BoundsHit)
	}
	if got := finalOutcomes(report); len(got) != 1 || !strings.Contains(got[0], "ticks = 0") {
		t.Fatalf("finals = %v, want ticks = 0 alone", got)
	}
	if len(report.Scope) != 1 {
		t.Fatalf("scope = %v, want the armed timer to scope the outcome", report.Scope)
	}
}

// A machine resting on a signal it has not been sent has nothing a duration would
// run, so its outcome is unscoped.
func testInitialHorizonNothingPending(t *testing.T) {
	report := checkStopped(t, parseLibraryModel(t, initialHorizonModel), "Idle", HorizonInitial())
	if len(report.Scope) != 0 {
		t.Fatalf("scope = %v, want none", report.Scope)
	}
}

// The order a parallel state's regions are entered in is part of the initial
// transition, and the search still draws it.
func testInitialHorizonSearchesRegionEntry(t *testing.T) {
	report := checkStopped(t, parseLibraryModel(t, initialHorizonModel), "Pair", HorizonInitial())
	if len(report.Finals) != 6 {
		t.Fatalf("finals = %v, want the six entry orders", finalOutcomes(report))
	}
	if len(report.Scope) != 0 {
		t.Fatalf("scope = %v, want none", report.Scope)
	}
}

// An action has no initial transition to stop at: an invocation starting one
// under the initial horizon is refused before anything moves.
func testInitialHorizonRefusesAnAction(t *testing.T) {
	m := parseLibraryModel(t, initialHorizonModel)
	action, machine := m.action(t, "noop"), m.state(t, "Waits")
	start := func(ctx *Context) (*Invocation, error) {
		exec, err := ctx.CreateActionExecutor(action)
		if err != nil {
			return nil, err
		}
		state, err := ctx.CreateStateExecutor(machine)
		if err != nil {
			return nil, err
		}
		return &Invocation{Actions: []*ActionExecutor{exec}, States: []*StateExecutor{state}, Horizon: HorizonInitial()}, nil
	}
	_, err := Check(context.Background(), m.fresh, start, CheckBudget{}, unreduced(), nil)
	if !errors.Is(err, ErrNothingStarted) {
		t.Fatalf("err = %v, want ErrNothingStarted", err)
	}
}

// A horizon past the do body's wait runs it to its end, with no scope left.
func testAdvancedHorizonRunsTheDoBody(t *testing.T) {
	report := checkStopped(t, parseLibraryModel(t, initialHorizonModel), "Waits", HorizonAt(3))
	if got := finalOutcomes(report); len(got) != 1 || !strings.Contains(got[0], "c = 2") {
		t.Fatalf("finals = %v, want c = 2 alone", got)
	}
	if len(report.Scope) != 0 {
		t.Fatalf("scope = %v, want none", report.Scope)
	}
}
