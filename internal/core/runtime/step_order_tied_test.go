package runtime

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Of two time triggers tied at the head while a do step is due, the one whose dispatch
// acts is drawn against the step by name, the dropped one waits; every witness replays.
func TestStepOrderDrawsTheActingTiedEventAlone(t *testing.T) {
	text, err := os.ReadFile(filepath.Join("testdata", "conformance", "state_do_step_or_tied_dispatch.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	m := parseLibraryModel(t, strings.Replace(string(text), "package Test", "package test", 1))
	start := stateStarterOf(m.state(t, "Machine"), HorizonAt(3))
	report, err := Check(context.Background(), m.fresh, start, CheckBudget{}, unreduced(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != CheckDivergent || len(report.Violations) != 0 || len(report.BoundsHit) != 0 || len(report.NotEnumerated) != 0 {
		t.Fatalf("check: %s, want a complete divergent search", report.Status())
	}
	want := []string{`finalState idle; visits top, idle; log = "did one "`, `finalState idle; visits top, idle; log = "did two "`, `finalState idle; visits top, idle; log = "two "`}
	if got := finalOutcomes(report); !slices.Equal(got, want) {
		t.Fatalf("finals %v, want the step first under either trigger and the unguarded trigger first", got)
	}
	for _, final := range report.Finals {
		steps := slices.DeleteFunc(slices.Clone(final.Witness.Choices), func(c ChoiceTaken) bool { return c.Kind != ChoiceStepOrder })
		if len(steps) != 1 || !slices.Equal(steps[0].Among, []string{"do top", "dispatch time top 2->idle"}) {
			t.Fatalf("%s: step orders %v, want one between the step and the unguarded trigger alone", final.Outcome, steps)
		}
		dispatched := slices.ContainsFunc(final.Witness.Choices, func(c ChoiceTaken) bool { return c.Kind == ChoiceDispatchOrder })
		if before := steps[0].Took != "do top"; before == dispatched {
			t.Fatalf("%s: the dispatch order is drawn exactly when the step goes first, got %s", final.Outcome, FormatChoices(final.Witness.Choices))
		}
		r := replayWitness(t, m, start, final.Witness, final.Outcome)
		if got := r.Ctx.Trace().String(); got != final.Witness.Trace {
			t.Errorf("replay of %s left the trace\n%s\nwant the witness's\n%s", final.Outcome, got, final.Witness.Trace)
		}
	}
}
