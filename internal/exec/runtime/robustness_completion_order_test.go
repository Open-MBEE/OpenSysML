package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// completionOrderModel enters a parallel state on Go whose two regions start in
// states that complete at once and perform nothing; the completions' effects log
// the order the pool dispatched them, which is the order the entries were drawn.
const completionOrderModel = `package test {
	private import ScalarValues::*;
	attribute def Go;
	state def Machine {
		attribute log : String = "";
		entry; then idle;
		state idle;
		transition first idle accept Go then work;
		state work parallel {
			state left {
				entry; then l1;
				state l1;
				state l2;
				transition first l1 do { assign log := log + "l "; } then l2;
			}
			state right {
				entry; then r1;
				state r1;
				state r2;
				transition first r1 do { assign log := log + "r "; } then r2;
			}
		}
	}
}`

// TestRuntimeRobustnessCompletionOrder: a completing entry is a drawn unit, so a
// witness naming one the front does not hold is a typed refusal before any
// completion is queued, a witness with a move the pool's order never offers is
// left over rather than followed, and a front of completing entries beyond the
// budget is no hang.
func TestRuntimeRobustnessCompletionOrder(t *testing.T) {
	t.Run("entry_order_naming_a_completing_state_the_front_does_not_hold", testEntryOrderNamingACompletingStateTheFrontDoesNotHold)
	t.Run("completions_dispatched_in_the_replayed_draws_order", testCompletionsDispatchedInTheReplayedDrawsOrder)
	t.Run("witness_ordering_the_pool_after_the_draw_is_left_over", testWitnessOrderingThePoolAfterTheDrawIsLeftOver)
	t.Run("wide_completing_front_beyond_the_run_budget", testWideCompletingFrontBeyondTheRunBudget)
}

// completionOrderReplay drives completionOrderModel under the witness through one
// Go and returns the log, the run's error and the witness's unfollowed moves.
func completionOrderReplay(t *testing.T, lines string) (string, error, error) {
	t.Helper()
	m := parseExploreModel(t, completionOrderModel)
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
	exec.SendSignal("Go", nil)
	err = exec.RunToCompletion()
	return FormatValue(exec.StateData()["log"]), err, ctx.Unfollowed()
}

// testEntryOrderNamingACompletingStateTheFrontDoesNotHold: an entry-order line
// naming a state no region is about to enter is refused at the draw, before
// either entry runs or either completion is queued.
func testEntryOrderNamingACompletingStateTheFrontDoesNotHold(t *testing.T) {
	log, err, _ := completionOrderReplay(t, "entering work: zork(entry) first of l1(entry), zork(entry)\n")
	var refused *ReplayError
	if !errors.As(err, &refused) || !errors.Is(err, ErrReplayRefused) {
		t.Fatalf("error %T %v, want a ReplayError", err, err)
	}
	if refused.Move != 1 || !strings.Contains(refused.Error(), "zork(entry) is not enabled (enabled: l1(entry), r1(entry))") {
		t.Errorf("refused %v, want move 1 naming zork(entry) as not enabled among l1(entry), r1(entry)", refused)
	}
	if log != `""` {
		t.Errorf("log is %s after the refusal, want empty: no completion may dispatch on a refused draw", log)
	}
}

// testCompletionsDispatchedInTheReplayedDrawsOrder: the witness's entry draw
// decides the pool's order, so replaying either draw dispatches the completions
// in that draw's order, with no move left over — the pool's order is no choice.
func testCompletionsDispatchedInTheReplayedDrawsOrder(t *testing.T) {
	for _, tc := range []struct{ first, log string }{
		{"l1", `"l r "`},
		{"r1", `"r l "`},
	} {
		log, err, unfollowed := completionOrderReplay(t, "entering work: "+tc.first+"(entry) first of l1(entry), r1(entry)\n")
		if err != nil || unfollowed != nil {
			t.Fatalf("%s first: run %v, unfollowed %v, want the witness followed whole", tc.first, err, unfollowed)
		}
		if log != tc.log {
			t.Errorf("%s first: log is %s, want %s: completions dispatch in the entry draw's order", tc.first, log, tc.log)
		}
	}
}

// testWitnessOrderingThePoolAfterTheDrawIsLeftOver: a second entry-order line
// after the one draw the run makes names a choice the pool's fixed order never
// offers, so the run ends with it left over, having followed the first.
func testWitnessOrderingThePoolAfterTheDrawIsLeftOver(t *testing.T) {
	log, err, unfollowed := completionOrderReplay(t,
		"entering work: r1(entry) first of l1(entry), r1(entry)\n"+
			"entering work: l1(entry) first of l1(entry), r1(entry)\n")
	if err != nil {
		t.Fatalf("run: %v, want it to end on its own", err)
	}
	var refused *ReplayError
	if !errors.As(unfollowed, &refused) || !errors.Is(unfollowed, ErrReplayRefused) {
		t.Fatalf("unfollowed %T %v, want a ReplayError", unfollowed, unfollowed)
	}
	if refused.Move != 2 || !strings.Contains(refused.Error(), "the run ended") {
		t.Errorf("refused %v, want move 2 left over when the run ended", refused)
	}
	if log != `"r l "` {
		t.Errorf("log is %s, want the first draw followed: r1 entered first, its completion dispatched first", log)
	}
}

// testWideCompletingFrontBeyondTheRunBudget: a parallel state three deep and three
// wide whose leaves all complete at once has more entry linearizations than the
// budget's runs; the exploration reports that, the same way twice.
func testWideCompletingFrontBeyondTheRunBudget(t *testing.T) {
	region := func(name string) string {
		return "state " + name + " { entry; then s; state s; state d; transition first s do { assign n := n + 1; } then d; }\n"
	}
	wide := func(name string) string {
		return "state " + name + " { entry; then p; state p parallel {\n" + region(name+"1") + region(name+"2") + region(name+"3") + "} }\n"
	}
	m := parseExploreModel(t, `package test {
		private import ScalarValues::*;
		state def Machine {
			attribute n : Integer = 0;
			entry; then work;
			state work parallel {
				`+wide("a")+wide("b")+wide("c")+`
			}
		}
	}`)
	sym := m.state(t, "Machine")
	run := stateRun(sym, "")
	policy, err := ParseSchedulePolicy("explore")
	if err != nil {
		t.Fatal(err)
	}
	var first *Exploration
	for i := 0; i < 2; i++ {
		x, err := Explore(context.Background(), policy, m.fresh, run)
		if err != nil {
			t.Fatalf("explore: %v", err)
		}
		if x.Complete() || !strings.Contains(x.Status(), "runs budget") {
			t.Fatalf("status %q, want the runs budget hit", x.Status())
		}
		if x.Runs != DefaultExploreBudget.Runs {
			t.Errorf("exploration made %d runs, want the budget's %d", x.Runs, DefaultExploreBudget.Runs)
		}
		if first == nil {
			first = x
			continue
		}
		if a, b := explored(t, first), explored(t, x); strings.Join(a, "\n") != strings.Join(b, "\n") {
			t.Errorf("two explorations of one model differ")
		}
	}
}
