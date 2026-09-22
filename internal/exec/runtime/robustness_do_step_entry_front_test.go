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

// TestRuntimeRobustnessDoStepEntryFront: a do step drawn against a sibling's entry unit
// ends at the entry or a named budget, is offered only once its state's entry has performed
// and only while a move is due, and never under a fixed policy.
func TestRuntimeRobustnessDoStepEntryFront(t *testing.T) {
	t.Run("busy_do_flow_against_a_sibling_entry_ends_at_the_entry_or_the_budget", testBusyDoFlowAgainstASiblingEntry)
	t.Run("do_step_named_before_its_entry_has_performed_is_refused", testDoStepNamedBeforeItsEntryIsRefused)
	t.Run("do_body_parked_at_an_accept_offers_no_draw_on_the_front", testParkedDoBodyOffersNoDrawOnTheFront)
	t.Run("fixed_policies_run_the_entries_whole_and_the_do_round_after", testFixedPoliciesRunTheEntriesWhole)
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
