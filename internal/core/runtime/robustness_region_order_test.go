package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// regionOrderModel enters a parallel state on Go whose two regions log their
// entries and exits and each fire on the next Go, so every drawn unit shows in log.
const regionOrderModel = `package test {
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
				state l1 {
					entry { assign log := log + "l1(entry) "; }
					exit { assign log := log + "l1(exit) "; }
				}
				state l2 { exit { assign log := log + "l2(exit) "; } }
				transition first l1 accept Go do { assign log := log + "T1(effect) "; } then l2;
			}
			state right {
				entry; then r1;
				state r1 {
					entry { assign log := log + "r1(entry) "; }
					exit { assign log := log + "r1(exit) "; }
				}
				state r2 { exit { assign log := log + "r2(exit) "; } }
				transition first r1 accept Go do { assign log := log + "T2(effect) "; } then r2;
			}
		}
		state rest;
		transition first work accept Go then rest;
	}
}`

// TestRuntimeRobustnessRegionOrder exercises the failure modes of the drawn
// order of orthogonal regions: a witness line naming a unit the front does not
// hold is a typed refusal that leaves the machine's data as the move found it,
// and a front too wide for the budget is an incomplete exploration, not a hang.
func TestRuntimeRobustnessRegionOrder(t *testing.T) {
	t.Run("entry_order_naming_a_region_the_front_does_not_hold", testEntryOrderNamingARegionTheFrontDoesNotHold)
	t.Run("firing_unit_order_naming_a_unit_of_no_firing", testFiringUnitOrderNamingAUnitOfNoFiring)
	t.Run("exit_order_naming_a_state_not_being_left", testExitOrderNamingAStateNotBeingLeft)
	t.Run("deep_wide_front_beyond_the_run_budget", testDeepWideFrontBeyondTheRunBudget)
}

// refusedRegionOrderReplay drives regionOrderModel under the witness through the
// signals given and returns the refusal, with the log the refusal left behind.
func refusedRegionOrderReplay(t *testing.T, lines string, signals int) (*ReplayError, string) {
	t.Helper()
	m := parseExploreModel(t, regionOrderModel)
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
	for i := 0; i < signals; i++ {
		exec.SendSignal("Go", nil)
	}
	err = exec.RunToCompletion()
	var refused *ReplayError
	if !errors.As(err, &refused) || !errors.Is(err, ErrReplayRefused) {
		t.Fatalf("error %T %v, want a ReplayError", err, err)
	}
	return refused, FormatValue(exec.StateData()["log"])
}

// testEntryOrderNamingARegionTheFrontDoesNotHold: an entry-order line naming a
// state no region is about to enter is refused at the draw, before either
// region's entry runs.
func testEntryOrderNamingARegionTheFrontDoesNotHold(t *testing.T) {
	refused, log := refusedRegionOrderReplay(t, "entering work: zork(entry) first of l1(entry), zork(entry)\n", 1)
	if refused.Move != 1 || !strings.Contains(refused.Error(), "zork(entry) is not enabled (enabled: l1(entry), r1(entry))") {
		t.Errorf("refused %v, want move 1 naming zork(entry) as not enabled among l1(entry), r1(entry)", refused)
	}
	if log != `""` {
		t.Errorf("log is %s after the refusal, want empty: no entry may run on a refused draw", log)
	}
}

// testFiringUnitOrderNamingAUnitOfNoFiring: a firing-unit line naming a unit of
// no selected firing is refused before either source is left.
func testFiringUnitOrderNamingAUnitOfNoFiring(t *testing.T) {
	refused, log := refusedRegionOrderReplay(t,
		"entering work: l1(entry) first of l1(entry), r1(entry)\n"+
			"on accept Go: T9(effect) first of l1(exit), T9(effect)\n", 2)
	if refused.Move != 2 || !strings.Contains(refused.Error(), "T9(effect) is not enabled (enabled: l1(exit), r1(exit))") {
		t.Errorf("refused %v, want move 2 naming T9(effect) as not enabled among l1(exit), r1(exit)", refused)
	}
	if log != `"l1(entry) r1(entry) "` {
		t.Errorf("log is %s after the refusal, want the entries alone: no exit may run on a refused draw", log)
	}
}

// testExitOrderNamingAStateNotBeingLeft: an exit-order line naming a state the
// transition does not leave is refused before either region's exit runs. l2's
// silent entry rides with its firing's effect, so no line names it.
func testExitOrderNamingAStateNotBeingLeft(t *testing.T) {
	refused, log := refusedRegionOrderReplay(t,
		"entering work: l1(entry) first of l1(entry), r1(entry)\n"+
			"on accept Go: l1(exit) first of l1(exit), r1(exit)\n"+
			"on accept Go: l1->l2(effect) first of l1->l2(effect), r1(exit)\n"+
			"exiting work: rest(exit) first of l2(exit), rest(exit)\n", 3)
	if refused.Move != 4 || !strings.Contains(refused.Error(), "rest(exit) is not enabled (enabled: l2(exit), r2(exit))") {
		t.Errorf("refused %v, want move 4 naming rest(exit) as not enabled among l2(exit), r2(exit)", refused)
	}
	if log != `"l1(entry) r1(entry) l1(exit) T1(effect) r1(exit) T2(effect) "` {
		t.Errorf("log is %s after the refusal, want no exit of l2 or r2: none may run on a refused draw", log)
	}
}

// testDeepWideFrontBeyondTheRunBudget: a parallel state three deep and three
// wide has more entry linearizations than the default budget's runs; the
// exploration reports the budget hit, the same way twice, rather than hang.
func testDeepWideFrontBeyondTheRunBudget(t *testing.T) {
	region := func(name string) string {
		return "state " + name + " { entry; then s; state s { entry { assign n := n + 1; } } }\n"
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
