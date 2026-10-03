package runtime

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// TestRuntimeRobustnessExploreStatementOrder exercises the order of a body's direct
// statements no succession orders: every surface that enumerates or replays
// schedules reaches each order, a fixed policy keeps declaration order, and a
// search cut short by its budget says so.
func TestRuntimeRobustnessExploreStatementOrder(t *testing.T) {
	t.Run("explore_reaches_every_admitted_order", testStatementOrderExplored)
	t.Run("budget_exhaustion_is_incomplete", testStatementOrderBudgetHit)
	t.Run("recorded_order_replays", testStatementOrderReplay)
	t.Run("seeded_runs_are_reproducible_and_reach_each_order", testStatementOrderSeeded)
	t.Run("declared_and_reverse_keep_declaration_order", testStatementOrderFixedPolicies)
	t.Run("cyclic_calc_binding_is_a_typed_error", testStatementOrderCyclicCalcBinding)
}

const manyStatements = `package test {
	private import ScalarValues::*;
	action def Many {
		attribute c : Integer := 1;
		first start then s;
		action s {
			assign c := c * 2;
			assign c := c + 1;
			assign c := c * 3;
			assign c := c + 5;
			assign c := c * 7;
		}
		then done;
	}
}`

func testStatementOrderExplored(t *testing.T) {
	for _, c := range []struct {
		fixture, action, feature string
		want                     []string
	}{
		{"action_explore_statement_order_dependent", "Order", "y", []string{"0", "1"}},
		{"action_explore_statement_order_then", "Then", "y", []string{"1"}},
	} {
		t.Run(c.action, func(t *testing.T) {
			x := conformanceModel(t, c.fixture).exploreAction(t, "explore", c.action)
			if !x.Complete() {
				t.Fatalf("exploration %s, want complete", x.Status())
			}
			got := featureValues(t, x, c.feature)
			slices.Sort(got)
			if got = slices.Compact(got); !slices.Equal(got, c.want) {
				t.Fatalf("%s over every schedule = %v, want %v", c.feature, got, c.want)
			}
		})
	}
}

// testStatementOrderBudgetHit: five pairwise dependent statements have 120 orders;
// a runs budget below that ends the search incomplete, never complete over fewer.
func testStatementOrderBudgetHit(t *testing.T) {
	m := parseExploreModel(t, manyStatements)
	x := m.exploreAction(t, "explore:runs=8", "Many")
	if x.Complete() || !slices.Contains(x.BudgetsHit, "runs") || x.Runs != 8 {
		t.Fatalf("exploration %s after %d runs, want the runs budget of 8 hit", x.Status(), x.Runs)
	}
	if !strings.HasPrefix(x.Status(), "incomplete") || !strings.Contains(x.Status(), "runs budget 8") {
		t.Errorf("status %q, want incomplete naming the runs budget", x.Status())
	}
	x = m.exploreAction(t, "explore", "Many")
	if !x.Complete() || x.Runs != 120 {
		t.Fatalf("exploration %s after %d runs, want complete over the 120 orders", x.Status(), x.Runs)
	}
}

func testStatementOrderReplay(t *testing.T) {
	m := conformanceModel(t, "action_explore_statement_order_dependent")
	run := caseRun(t, m, "Order")
	x := m.exploreAction(t, "explore", "Order")
	i := slices.IndexFunc(x.Outcomes, func(o ExploredOutcome) bool { return outcomeValue(t, o.Outcome, "y") == "0" })
	if i < 0 {
		t.Fatalf("exploration reached %v, want y = 0, read before the write", outcomeTexts(x))
	}
	recorded := FormatChoices(x.Outcomes[i].Witness)
	parsed, err := ParseChoices(recorded)
	if err != nil {
		t.Fatalf("parse recorded witness %q: %v", recorded, err)
	}
	outcome, taken, err := replayed(t, m.fresh, run, parsed)
	if err != nil {
		t.Fatalf("replay %q: %v", recorded, err)
	}
	if got := outcomeValue(t, outcome, "y"); got != "0" {
		t.Fatalf("replaying %q gave y = %s, want 0", recorded, got)
	}
	if again := FormatChoices(taken); again != recorded {
		t.Errorf("replay took %q, want the witness %q", again, recorded)
	}
}

func testStatementOrderSeeded(t *testing.T) {
	m := conformanceModel(t, "action_explore_statement_order_dependent")
	run := caseRun(t, m, "Order")
	seen := map[string]bool{}
	for seed := 1; seed <= 32; seed++ {
		spelling := fmt.Sprintf("seed:%d", seed)
		var first string
		for range 2 {
			ctx, err := m.fresh()
			if err != nil {
				t.Fatal(err)
			}
			mustSchedule(t, ctx, mustPolicy(t, spelling))
			outcome, err := run(ctx)
			if err != nil {
				t.Fatalf("%s: %v", spelling, err)
			}
			got := outcomeValue(t, outcome, "y")
			if first == "" {
				first = got
			} else if got != first {
				t.Fatalf("%s gave y = %s, then y = %s; want the seed to replay its run", spelling, first, got)
			}
		}
		seen[first] = true
	}
	if !seen["0"] || !seen["1"] {
		t.Fatalf("seeds 1..32 reached %v, want both y = 0 and y = 1", seen)
	}
}

func testStatementOrderFixedPolicies(t *testing.T) {
	m := conformanceModel(t, "action_explore_statement_order_dependent")
	run := caseRun(t, m, "Order")
	for _, spelling := range []string{"reverse", "declared"} {
		ctx, err := m.fresh()
		if err != nil {
			t.Fatal(err)
		}
		mustSchedule(t, ctx, mustPolicy(t, spelling))
		outcome, err := run(ctx)
		if err != nil {
			t.Fatalf("%s: %v", spelling, err)
		}
		if got := outcomeValue(t, outcome, "y"); got != "1" {
			t.Errorf("%s gave y = %s, want 1, the statements in declaration order", spelling, got)
		}
	}
}

// cyclicCalcBindings binds a calc usage's input from itself, and two usages from each other.
const cyclicCalcBindings = `package test {
	private import ScalarValues::*;
	calc def Twice { in k : Real; out d = k * 2.0; }
	action def Self {
		attribute v : Real = 1.0;
		attribute doubled : Real = 0.0;
		first start then compute;
		action compute {
			calc t : Twice { in k = t.d; }
			assign v := 2.0;
			assign doubled := t.d;
		}
		then done;
	}
	action def Mutual {
		attribute v : Real = 1.0;
		attribute doubled : Real = 0.0;
		first start then compute;
		action compute {
			calc a : Twice { in k = b.d; }
			calc b : Twice { in k = a.d; }
			assign v := 2.0;
			assign doubled := a.d;
		}
		then done;
	}
}`

// testStatementOrderCyclicCalcBinding: building the statements' footprints through
// a cyclic binding terminates, and every order ends in the runtime's recursion error.
func testStatementOrderCyclicCalcBinding(t *testing.T) {
	m := parseExploreModel(t, cyclicCalcBindings)
	for _, action := range []string{"Self", "Mutual"} {
		t.Run(action, func(t *testing.T) {
			x := m.exploreAction(t, "explore", action)
			if !x.Complete() || len(x.Outcomes) == 0 {
				t.Fatalf("exploration %s with %v, want complete", x.Status(), outcomeTexts(x))
			}
			for _, o := range x.Outcomes {
				if !errors.Is(o.Outcome.Err, ErrCalcUsageRecursion) {
					t.Errorf("outcome %s, want ErrCalcUsageRecursion", o.Outcome)
				}
			}
		})
	}
}
