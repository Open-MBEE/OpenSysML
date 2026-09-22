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
// acts is drawn against each move of the step by name, the dropped one waits; once the
// step's move wrote what guards the other, both act and the tie is drawn; every witness replays.
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
	if report.Verdict != CheckDivergent || len(report.Violations) != 0 || len(report.BoundsHit) != 0 {
		t.Fatalf("check: %s, want a complete divergent search", report.Status())
	}
	want := []string{`finalState idle; visits top, idle; log = "did one "`, `finalState idle; visits top, idle; log = "did two "`, `finalState idle; visits top, idle; log = "two "`}
	if got := finalOutcomes(report); !slices.Equal(got, want) {
		t.Fatalf("finals %v, want the step first under either trigger and the unguarded trigger first", got)
	}
	unguarded := []string{"do top", "dispatch time top 2->idle"}
	tied := []string{"do top", dispatchTiedLabel}
	for _, final := range report.Finals {
		steps := slices.DeleteFunc(slices.Clone(final.Witness.Choices), func(c ChoiceTaken) bool { return c.Kind != ChoiceStepOrder })
		wrote := strings.Contains(final.Outcome, "did ")
		if len(steps) == 0 || !slices.Equal(steps[0].Among, unguarded) {
			t.Fatalf("%s: step orders %v, want the first between the step and the unguarded trigger alone", final.Outcome, steps)
		}
		firstTied := slices.IndexFunc(steps, func(c ChoiceTaken) bool { return slices.Equal(c.Among, tied) })
		for i, step := range steps {
			want := unguarded
			if firstTied >= 0 && i >= firstTied {
				want = tied
			}
			if !slices.Equal(step.Among, want) {
				t.Fatalf("%s: step order %d is %v, want %v: the tie is drawn once the write made both act", final.Outcome, i, step, want)
			}
			if i < len(steps)-1 && step.Took != "do top" {
				t.Fatalf("%s: step order %d took the dispatch before the last, got %s", final.Outcome, i, FormatChoices(final.Witness.Choices))
			}
		}
		if wrote != (firstTied >= 0) {
			t.Fatalf("%s: the tie is drawn exactly when the step wrote, got %s", final.Outcome, FormatChoices(final.Witness.Choices))
		}
		dispatched := slices.ContainsFunc(final.Witness.Choices, func(c ChoiceTaken) bool { return c.Kind == ChoiceDispatchOrder })
		if wrote != dispatched {
			t.Fatalf("%s: the dispatch order is drawn exactly when the step wrote before the dispatch, got %s", final.Outcome, FormatChoices(final.Witness.Choices))
		}
		r := replayWitness(t, m, start, final.Witness, final.Outcome)
		if got := r.Ctx.Trace().String(); got != final.Witness.Trace {
			t.Errorf("replay of %s left the trace\n%s\nwant the witness's\n%s", final.Outcome, got, final.Witness.Trace)
		}
	}
}
