package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// busyDoOnEntryFrontModel spins the left region's do flow forever from the moment l1 is
// entered, while the right region's entry is still left on the front; r1's completion
// then leaves work, cutting the flow, if its dispatch is drawn ahead of a move.
const busyDoOnEntryFrontModel = `package test {
	private import ScalarValues::*;
	state def Machine {
		attribute n : Integer = 0;
		entry; then work;
		state work parallel {
			state left {
				entry; then l1;
				state l1 {
					do action spin { first start; then merge again; then action count assign n := n + 1; then again; }
				}
			}
			state right {
				entry; then r1;
				state r1;
				transition first r1 then idle;
			}
		}
		state idle;
	}
}`

// busyCompositeDoModel spins work's own do flow forever from the moment work is entered,
// while its body's entry is still on the way down; w1's completion then leaves work.
const busyCompositeDoModel = `package test {
	private import ScalarValues::*;
	state def Machine {
		attribute n : Integer = 0;
		entry; then work;
		state work {
			do action spin { first start; then merge again; then action count assign n := n + 1; then again; }
			entry; then w1;
			state w1;
			transition first w1 then idle;
		}
		state idle;
	}
}`

// TestRuntimeRobustnessDoStepEntryFront: a do step drawn against a sibling's entry unit
// ends at the entry or a named budget, is offered only once its state's entry has performed
// and only while a move is due, and never under a fixed policy; a composite's own step is
// drawn against its substates' entries the same way, on a front or down a serial body.
func TestRuntimeRobustnessDoStepEntryFront(t *testing.T) {
	t.Run("busy_do_flow_against_a_sibling_entry_ends_at_the_entry_or_the_budget", testBusyDoFlowAgainstASiblingEntry)
	t.Run("do_step_named_before_its_entry_has_performed_is_refused", testDoStepNamedBeforeItsEntryIsRefused)
	t.Run("do_body_parked_at_an_accept_offers_no_draw_on_the_front", testParkedDoBodyOffersNoDrawOnTheFront)
	t.Run("fixed_policies_run_the_entries_whole_and_the_do_round_after", testFixedPoliciesRunTheEntriesWhole)
	t.Run("busy_composite_do_flow_against_its_own_body_entry_ends_at_the_entry_or_the_budget", testBusyCompositeDoFlowAgainstItsOwnBodyEntry)
	t.Run("composite_do_step_named_before_the_composite_is_entered_is_refused", testCompositeDoStepNamedBeforeItsEntryIsRefused)
	t.Run("composite_do_step_replays_before_between_and_after_its_own_body_entries", testCompositeDoStepReplaysAlongItsOwnBody)
	t.Run("composite_do_behavior_ending_ahead_of_its_body_completes_nothing", testCompositeDoEndingAheadOfItsBodyCompletesNothing)
	t.Run("fixed_policies_run_a_composite_body_whole_before_its_own_do_round", testFixedPoliciesRunACompositeBodyWhole)
}

// testBusyDoFlowAgainstASiblingEntry: every explored run of an endless do flow on the entry
// front enters the sibling after some moves and is cut by its completion, or hits the
// do-step budget, a typed error.
func testBusyDoFlowAgainstASiblingEntry(t *testing.T) {
	m := parseExploreModel(t, busyDoOnEntryFrontModel)
	sym := m.state(t, "Machine")
	run := func(ctx *Context) (Outcome, error) {
		budgets := ctx.Budgets()
		budgets.MaxDoSteps = 6
		if err := ctx.SetBudgets(budgets); err != nil {
			return Outcome{}, err
		}
		exec, err := ctx.CreateStateExecutor(sym)
		if err != nil {
			return Outcome{}, err
		}
		err = exec.RunToCompletion()
		return exec.Outcome(), err
	}
	x, err := Explore(context.Background(), mustPolicy(t, "explore"), m.fresh, run)
	if err != nil {
		t.Fatalf("explore: %v", err)
	}
	if !x.Complete() || len(x.Outcomes) < 2 {
		t.Fatalf("exploration %s with %d outcomes, want complete with entered and budgeted runs", x.Status(), len(x.Outcomes))
	}
	var entered, budgeted int
	for _, o := range x.Outcomes {
		switch {
		case errors.Is(o.Outcome.Err, ErrDoStepLimitExceeded):
			budgeted++
		case o.Outcome.Err == nil && o.Outcome.FinalState == "idle":
			entered++
		default:
			t.Errorf("outcome %s: error %v, want the sibling's completion taken or the do-step budget", o.Outcome, o.Outcome.Err)
		}
	}
	if entered == 0 || budgeted == 0 {
		t.Errorf("%d entered, %d budgeted runs, want both", entered, budgeted)
	}
}

// testDoStepNamedBeforeItsEntryIsRefused: the front's first draw is between the two entries;
// a witness naming the do step there is refused, with the alternatives the run faced.
func testDoStepNamedBeforeItsEntryIsRefused(t *testing.T) {
	m := parseExploreModel(t, busyDoOnEntryFrontModel)
	sym := m.state(t, "Machine")
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	witness, err := ParseChoices("entering work: do l1 first of do l1, r1(entry)\n")
	if err != nil {
		t.Fatal(err)
	}
	mustSchedule(t, ctx, ReplayPolicy(witness))
	exec, err := ctx.CreateStateExecutor(sym)
	if err == nil {
		err = exec.RunToCompletion()
	}
	var refused *ReplayError
	if !errors.As(err, &refused) || refused.Move != 1 {
		t.Fatalf("error %T %v, want the first line refused", err, err)
	}
	if got := refused.Error(); !strings.Contains(got, "l1(entry), r1(entry)") {
		t.Errorf("refusal %q does not name the entries the run faced", got)
	}
}

// testParkedDoBodyOffersNoDrawOnTheFront: a do body's one move to an accept of a signal not
// in the pool parks it with no move due, so the front draws the sibling's entry alone and
// a third line naming the do step finds no draw: refused as unfollowed.
func testParkedDoBodyOffersNoDrawOnTheFront(t *testing.T) {
	m := parseExploreModel(t, `package test {
		private import ScalarValues::*;
		attribute def Go;
		state def Machine {
			attribute log : String = "";
			entry; then work;
			state work parallel {
				state left {
					entry; then l1;
					state l1 {
						do action wait { first start; then action w accept Go; then action mark assign log := log + "go "; then done; }
					}
				}
				state right {
					entry; then r1;
					state r1 {
						entry { assign log := log + "r1(entry) "; }
					}
				}
			}
		}
	}`)
	sym := m.state(t, "Machine")
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	witness, err := ParseChoices("entering work: l1(entry) first of l1(entry), r1(entry)\nentering work: do l1 first of do l1, r1(entry)\nentering work: do l1 first of do l1, r1(entry)\n")
	if err != nil {
		t.Fatal(err)
	}
	mustSchedule(t, ctx, ReplayPolicy(witness))
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		t.Fatal(err)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if log := FormatValue(exec.StateData()["log"]); log != `"r1(entry) "` {
		t.Errorf("log is %s, want the entry alone with the body parked", log)
	}
	err = ctx.Unfollowed()
	var refused *ReplayError
	if !errors.As(err, &refused) || refused.Move != 3 {
		t.Fatalf("error %T %v, want the third line refused as unfollowed", err, err)
	}
}

// testFixedPoliciesRunTheEntriesWhole: declared, reverse and a seed run every entry before
// any do move, so the do flow, given a step budget of one round, never runs into it.
func testFixedPoliciesRunTheEntriesWhole(t *testing.T) {
	m := parseExploreModel(t, `package test {
		private import ScalarValues::*;
		state def Machine {
			attribute log : String = "";
			entry; then work;
			state work parallel {
				state left {
					entry; then l1;
					state l1 {
						do { assign log := log + "did "; }
					}
				}
				state right {
					entry; then r1;
					state r1 {
						entry { assign log := log + "r1(entry) "; }
					}
				}
			}
		}
	}`)
	sym := m.state(t, "Machine")
	for _, policy := range []string{"declared", "reverse", "seed:1", "seed:7"} {
		ctx, err := m.fresh()
		if err != nil {
			t.Fatal(err)
		}
		mustSchedule(t, ctx, mustPolicy(t, policy))
		exec, err := ctx.CreateStateExecutor(sym)
		if err != nil {
			t.Fatal(err)
		}
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("%s: run: %v", policy, err)
		}
		if log := FormatValue(exec.StateData()["log"]); log != `"r1(entry) did "` {
			t.Errorf("%s: log is %s, want the entry before the do round", policy, log)
		}
	}
}

// testBusyCompositeDoFlowAgainstItsOwnBodyEntry: every explored run of an endless do flow
// of the composite itself enters its body after some moves and is cut by the body's
// completion, or hits the do-step budget, a typed error.
func testBusyCompositeDoFlowAgainstItsOwnBodyEntry(t *testing.T) {
	m := parseExploreModel(t, busyCompositeDoModel)
	sym := m.state(t, "Machine")
	run := func(ctx *Context) (Outcome, error) {
		budgets := ctx.Budgets()
		budgets.MaxDoSteps = 6
		if err := ctx.SetBudgets(budgets); err != nil {
			return Outcome{}, err
		}
		exec, err := ctx.CreateStateExecutor(sym)
		if err != nil {
			return Outcome{}, err
		}
		err = exec.RunToCompletion()
		return exec.Outcome(), err
	}
	x, err := Explore(context.Background(), mustPolicy(t, "explore"), m.fresh, run)
	if err != nil {
		t.Fatalf("explore: %v", err)
	}
	if !x.Complete() || len(x.Outcomes) < 2 {
		t.Fatalf("exploration %s with %d outcomes, want complete with entered and budgeted runs", x.Status(), len(x.Outcomes))
	}
	var entered, budgeted int
	for _, o := range x.Outcomes {
		switch {
		case errors.Is(o.Outcome.Err, ErrDoStepLimitExceeded):
			budgeted++
		case o.Outcome.Err == nil && o.Outcome.FinalState == "idle":
			entered++
		default:
			t.Errorf("outcome %s: error %v, want the body's completion taken or the do-step budget", o.Outcome, o.Outcome.Err)
		}
		if len(o.Witness) > 0 && !strings.HasPrefix(o.Witness[0].String(), "entering work: ") {
			t.Errorf("outcome %s: first draw %s, want it at work's entry", o.Outcome, o.Witness[0])
		}
	}
	if entered == 0 || budgeted == 0 {
		t.Errorf("%d entered, %d budgeted runs, want both", entered, budgeted)
	}
}

// testCompositeDoStepNamedBeforeItsEntryIsRefused: the machine's entry of work draws
// nothing, no do behavior being due there; a witness naming work's step at that site is
// refused with the draw the run made instead, at work's own entry.
func testCompositeDoStepNamedBeforeItsEntryIsRefused(t *testing.T) {
	m := parseExploreModel(t, busyCompositeDoModel)
	sym := m.state(t, "Machine")
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	witness, err := ParseChoices("entering Machine: do work first of do work, work(entry)\n")
	if err != nil {
		t.Fatal(err)
	}
	mustSchedule(t, ctx, ReplayPolicy(witness))
	exec, err := ctx.CreateStateExecutor(sym)
	if err == nil {
		err = exec.RunToCompletion()
	}
	var refused *ReplayError
	if !errors.As(err, &refused) || refused.Move != 1 {
		t.Fatalf("error %T %v, want the first line refused", err, err)
	}
	if got := refused.Error(); !strings.Contains(got, "entering work") || !strings.Contains(got, "do work, w1(entry)") {
		t.Errorf("refusal %q does not name the draw the run faced", got)
	}
}

// testCompositeDoStepReplaysAlongItsOwnBody: down a serial body no front orders, each
// entry on the way is a draw against the composite's due step, and a witness places the
// step before, between or after the two entries, each replayed to its own log.
func testCompositeDoStepReplaysAlongItsOwnBody(t *testing.T) {
	m := parseExploreModel(t, `package test {
		private import ScalarValues::*;
		state def Machine {
			attribute log : String = "";
			entry; then work;
			state work {
				do { assign log := log + "did "; }
				entry; then w1;
				state w1 {
					entry action { assign log := log + "w1(entry) "; } then w2;
					state w2 {
						entry { assign log := log + "w2(entry) "; }
					}
				}
			}
		}
	}`)
	sym := m.state(t, "Machine")
	cases := []struct{ lines, log string }{
		{"entering work: do work first of do work, w1(entry)\n", `"did w1(entry) w2(entry) "`},
		{"entering work: w1(entry) first of do work, w1(entry)\nentering w1: do work first of do work, w2(entry)\n", `"w1(entry) did w2(entry) "`},
		{"entering work: w1(entry) first of do work, w1(entry)\nentering w1: w2(entry) first of do work, w2(entry)\n", `"w1(entry) w2(entry) did "`},
	}
	for _, c := range cases {
		ctx, err := m.fresh()
		if err != nil {
			t.Fatal(err)
		}
		witness, err := ParseChoices(c.lines)
		if err != nil {
			t.Fatal(err)
		}
		mustSchedule(t, ctx, ReplayPolicy(witness))
		exec, err := ctx.CreateStateExecutor(sym)
		if err != nil {
			t.Fatal(err)
		}
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("%q: run: %v", c.lines, err)
		}
		if err := ctx.Unfollowed(); err != nil {
			t.Errorf("%q: %v, want every line followed", c.lines, err)
		}
		if log := FormatValue(exec.StateData()["log"]); log != c.log {
			t.Errorf("%q: log is %s, want %s", c.lines, log, c.log)
		}
	}
}

// testCompositeDoEndingAheadOfItsBodyCompletesNothing: a composite whose do behavior ends
// before its body is entered completes only once the body reaches `done`, not at the end
// of the do behavior.
func testCompositeDoEndingAheadOfItsBodyCompletesNothing(t *testing.T) {
	m := parseExploreModel(t, `package test {
		private import ScalarValues::*;
		attribute def Go;
		state def Machine {
			attribute log : String = "";
			entry; then work;
			state work {
				do { assign log := log + "did "; }
				entry; then w1;
				state w1 {
					entry { assign log := log + "w1(entry) "; }
				}
				transition first w1 accept Go then done;
			}
			state idle {
				entry { assign log := log + "idle "; }
			}
			transition first work then idle;
		}
	}`)
	sym := m.state(t, "Machine")
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	witness, err := ParseChoices("entering work: do work first of do work, w1(entry)\n")
	if err != nil {
		t.Fatal(err)
	}
	mustSchedule(t, ctx, ReplayPolicy(witness))
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		t.Fatal(err)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := ctx.Unfollowed(); err != nil {
		t.Errorf("%v, want the do step drawn ahead of the body's entry", err)
	}
	if got := exec.FinalStateName(); got != "w1" {
		t.Fatalf("work completed at the end of its do behavior: resting in %q, want w1", got)
	}
	exec.SendSignal("Go", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run after Go: %v", err)
	}
	if log, want := FormatValue(exec.StateData()["log"]), `"did w1(entry) idle "`; log != want {
		t.Errorf("log is %s, want %s", log, want)
	}
}

// testFixedPoliciesRunACompositeBodyWhole: declared, reverse and a seed enter a composite's
// substates whole, on a front or down a serial body, and run its own do round after.
func testFixedPoliciesRunACompositeBodyWhole(t *testing.T) {
	m := parseExploreModel(t, `package test {
		private import ScalarValues::*;
		state def Machine {
			attribute log : String = "";
			entry; then work;
			state work parallel {
				do { assign log := log + "did "; }
				state left {
					entry; then l1;
					state l1 {
						entry action { assign log := log + "l1(entry) "; } then l2;
						state l2 {
							entry { assign log := log + "l2(entry) "; }
						}
					}
				}
				state right {
					entry; then r1;
					state r1 {
						entry { assign log := log + "r1(entry) "; }
					}
				}
			}
		}
	}`)
	sym := m.state(t, "Machine")
	for _, policy := range []string{"declared", "reverse", "seed:1", "seed:7"} {
		ctx, err := m.fresh()
		if err != nil {
			t.Fatal(err)
		}
		mustSchedule(t, ctx, mustPolicy(t, policy))
		exec, err := ctx.CreateStateExecutor(sym)
		if err != nil {
			t.Fatal(err)
		}
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("%s: run: %v", policy, err)
		}
		log := FormatValue(exec.StateData()["log"])
		if !strings.HasSuffix(log, `did "`) || strings.Count(log, "(entry) ") != 3 {
			t.Errorf("%s: log is %s, want the three entries before the do round", policy, log)
		}
	}
}
