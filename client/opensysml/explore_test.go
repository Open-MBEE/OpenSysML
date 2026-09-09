package opensysml_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

const exploreSource = scheduleSource + `
package Explored {
	private import ScalarValues::*;

	action three {
		attribute winner : Integer = 0;
		first start;
		fork split;
		action a { assign winner := 1; }
		action b { assign winner := 2; }
		action c { assign winner := 3; }
		join sync;
		done;
		succession first start then split;
		succession first split then a;
		succession first split then b;
		succession first split then c;
		succession first a then sync;
		succession first b then sync;
		succession first c then sync;
		succession first sync then done;
	}

	action straight {
		attribute total : Integer = 0;
		first start;
		action one { assign total := total + 1; }
		action two { assign total := total + 2; }
		done;
		succession first start then one;
		succession first one then two;
		succession first two then done;
	}

	action shaky {
		attribute q : Real = 0.0;
		attribute d : Integer = 0;
		first start;
		fork split;
		action zero { assign d := 0; }
		action one { assign d := 1; }
		join sync;
		action divide { assign q := 1.0 / d; }
		done;
		succession first start then split;
		succession first split then zero;
		succession first split then one;
		succession first zero then sync;
		succession first one then sync;
		succession first sync then divide;
		succession first divide then done;
	}
}`

func winners(t *testing.T, exploration *opensysml.Exploration) string {
	t.Helper()
	parts := make([]string, 0, len(exploration.Outcomes))
	for _, outcome := range exploration.Outcomes {
		parts = append(parts, fmt.Sprintf("%d x%d", winner(t, outcome.Outputs), outcome.Linearizations))
	}
	return strings.Join(parts, "; ")
}

// Exploring reaches every outcome a choice point leads to, in canonical order,
// counts the linearizations reaching each, and names one run's choices.
func TestExploreActionReportsEveryOutcome(t *testing.T) {
	ctx := context.Background()
	client := newClient(t)
	model := parse(t, client, exploreSource)

	exploration, err := client.ExploreAction(ctx, model, "Explored::three", nil)
	if err != nil {
		t.Fatalf("ExploreAction: %v", err)
	}
	if got := winners(t, exploration); got != "1 x2; 2 x2; 3 x2" {
		t.Errorf("outcomes = %q, want each winner reached by two orders", got)
	}
	if !exploration.Complete || exploration.Runs != 6 || exploration.Status() != "complete (6 runs)" {
		t.Errorf("exploration = %+v, want complete after six runs", exploration)
	}
	if exploration.RunsBudget != 1024 || exploration.DepthBudget != 64 {
		t.Errorf("budget = %d runs, %d deep, want the defaults 1024 and 64", exploration.RunsBudget, exploration.DepthBudget)
	}
	for _, outcome := range exploration.Outcomes {
		if len(outcome.Witness) != 2 || !strings.HasPrefix(outcome.Witness[0], "step 3: ") {
			t.Errorf("winner %d witness = %q, want two token choices", winner(t, outcome.Outputs), outcome.Witness)
		}
		if outcome.Failed() {
			t.Errorf("winner %d failed: %s", winner(t, outcome.Outputs), outcome.Error)
		}
	}

	again, err := client.ExploreAction(ctx, model, "Explored::three", nil, opensysml.WithSchedule("explore"))
	if err != nil {
		t.Fatalf("ExploreAction again: %v", err)
	}
	if !reflect.DeepEqual(again, exploration) {
		t.Errorf("exploring twice differed:\n%+v\n%+v", again, exploration)
	}
}

// A behavior without a choice point is one run and one outcome.
func TestExploringWithoutAChoiceIsOneRun(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, exploreSource)

	exploration, err := client.ExploreAction(context.Background(), model, "Explored::straight", nil)
	if err != nil {
		t.Fatalf("ExploreAction: %v", err)
	}
	if len(exploration.Outcomes) != 1 || exploration.Outcomes[0].Linearizations != 1 || len(exploration.Outcomes[0].Witness) != 0 {
		t.Fatalf("outcomes = %+v, want one, reached once, with no choices", exploration.Outcomes)
	}
	if got := exploration.Outcomes[0].Outputs["total"]; got != opensysml.Int(3) {
		t.Errorf("total = %#v, want Int(3)", got)
	}
	if exploration.Status() != "complete (1 runs)" {
		t.Errorf("status = %q, want complete after one run", exploration.Status())
	}
}

// A budget the search hits is reported by name, with what was found so far.
func TestExploringUnderABudgetIsIncomplete(t *testing.T) {
	ctx := context.Background()
	client := newClient(t)
	model := parse(t, client, exploreSource)

	for _, test := range []struct{ schedule, hit, status string }{
		{"explore:runs=2", "runs", "incomplete: runs budget 2 hit after 2 runs"},
		{"explore:depth=1", "depth", "incomplete: depth budget 1 hit after 3 runs"},
		{"explore:depth=1,runs=1", "runs,depth", "incomplete: runs budget 1 and depth budget 1 hit after 1 runs"},
	} {
		exploration, err := client.ExploreAction(ctx, model, "Explored::three", nil, opensysml.WithSchedule(test.schedule))
		if err != nil {
			t.Fatalf("ExploreAction under %q: %v", test.schedule, err)
		}
		if exploration.Complete || strings.Join(exploration.BudgetsHit, ",") != test.hit || exploration.Status() != test.status {
			t.Errorf("under %q: exploration = %+v, status %q, want %q", test.schedule, exploration, exploration.Status(), test.status)
		}
		if len(exploration.Outcomes) == 0 {
			t.Errorf("under %q: no outcomes, want those found before the budget", test.schedule)
		}
	}
}

// A run that fails under some orders is an outcome of its own, not a failure
// of the exploration.
func TestARunThatFailsIsAnOutcome(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, exploreSource)

	exploration, err := client.ExploreAction(context.Background(), model, "Explored::shaky", nil)
	if err != nil {
		t.Fatalf("ExploreAction: %v", err)
	}
	if len(exploration.Outcomes) != 2 {
		t.Fatalf("outcomes = %+v, want a failing one and a completing one", exploration.Outcomes)
	}
	var failed, completed int
	for _, outcome := range exploration.Outcomes {
		if outcome.Failed() {
			failed++
			if !strings.Contains(outcome.Error, "division by zero") || len(outcome.Outputs) != 0 {
				t.Errorf("failing outcome = %+v, want division by zero with no outputs", outcome)
			}
		} else {
			completed++
			if got := outcome.Outputs["q"]; got != opensysml.Real(1) {
				t.Errorf("q = %#v, want Real(1)", got)
			}
		}
	}
	if failed != 1 || completed != 1 {
		t.Errorf("%d failing and %d completing outcomes, want one each", failed, completed)
	}
}

// A state machine's outcome is where it rests and the states it visited.
func TestExploreStateReportsEveryRestingState(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, exploreSource)

	exploration, err := client.ExploreState(context.Background(), model, "Sched::Dispatcher", []string{"Go"})
	if err != nil {
		t.Fatalf("ExploreState: %v", err)
	}
	var got []string
	for _, outcome := range exploration.Outcomes {
		got = append(got, fmt.Sprintf("%s via %s x%d", outcome.FinalState, strings.Join(outcome.Visited, ","), outcome.Linearizations))
		if len(outcome.Witness) != 1 || !strings.HasPrefix(outcome.Witness[0], "state idle on accept Go") {
			t.Errorf("witness = %q, want the one transition choice", outcome.Witness)
		}
	}
	if want := []string{"high via idle,high x1", "low via idle,low x1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("outcomes = %q, want %q", got, want)
	}
	if exploration.Status() != "complete (2 runs)" {
		t.Errorf("status = %q, want complete after two runs", exploration.Status())
	}
}

// An analysis case's outcome carries its outputs and its verdicts together.
func TestExploreAnalysisReportsEveryOutcome(t *testing.T) {
	client := newClient(t)
	model := parse(t, client, exploreSource)

	exploration, err := client.ExploreAnalysis(context.Background(), model, "Sched::raced")
	if err != nil {
		t.Fatalf("ExploreAnalysis: %v", err)
	}
	if got := winners(t, exploration); got != "1 x1; 2 x1" {
		t.Errorf("outcomes = %q, want each branch's winner once", got)
	}
	if exploration.Status() != "complete (2 runs)" {
		t.Errorf("status = %q, want complete after two runs", exploration.Status())
	}
}

// An exploring policy is only accepted by the Explore calls, and only they
// accept one; each refusal names the call to use.
func TestExploringPoliciesBelongToTheExploreCalls(t *testing.T) {
	ctx := context.Background()
	client := newClient(t)
	model := parse(t, client, exploreSource)

	for op, call := range map[string]func(string) error{
		"ExecuteAction": func(policy string) error {
			_, err := client.ExecuteAction(ctx, model, "Sched::race", nil, opensysml.WithSchedule(policy))
			return err
		},
		"ExecuteState": func(policy string) error {
			_, err := client.ExecuteState(ctx, model, "Sched::Dispatcher", nil, opensysml.WithSchedule(policy))
			return err
		},
		"RunAnalysis": func(policy string) error {
			_, err := client.RunAnalysis(ctx, model, "Sched::raced", opensysml.Schedule(policy))
			return err
		},
	} {
		for _, policy := range []string{"explore", "explore:runs=2"} {
			err := call(policy)
			if !errors.Is(err, opensysml.CodeInvalidArgument) || !strings.Contains(err.Error(), "Explore") {
				t.Errorf("%s under %q: err = %v, want CodeInvalidArgument naming the Explore call", op, policy, err)
			}
		}
	}

	for op, call := range map[string]func(string) error{
		"ExploreAction": func(policy string) error {
			_, err := client.ExploreAction(ctx, model, "Sched::race", nil, opensysml.WithSchedule(policy))
			return err
		},
		"ExploreState": func(policy string) error {
			_, err := client.ExploreState(ctx, model, "Sched::Dispatcher", nil, opensysml.WithSchedule(policy))
			return err
		},
		"ExploreAnalysis": func(policy string) error {
			_, err := client.ExploreAnalysis(ctx, model, "Sched::raced", opensysml.Schedule(policy))
			return err
		},
	} {
		for _, policy := range []string{"declared", "seed:1"} {
			err := call(policy)
			if !errors.Is(err, opensysml.CodeInvalidArgument) || !strings.Contains(err.Error(), policy) {
				t.Errorf("%s under %q: err = %v, want CodeInvalidArgument naming it", op, policy, err)
			}
		}
		for _, policy := range []string{"explore:", "explore:runs=0", "explore:depth=-1", "explore:runs=x", "explores"} {
			err := call(policy)
			if !errors.Is(err, opensysml.CodeInvalidArgument) || !strings.Contains(err.Error(), policy) {
				t.Errorf("%s under %q: err = %v, want CodeInvalidArgument naming it", op, policy, err)
			}
		}
	}
}
