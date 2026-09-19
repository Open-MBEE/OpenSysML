package runtime

import (
	"context"
	"slices"
	"testing"
)

// pendingSignalModel is a machine whose do step is due as Stop or Go would leave
// the state, with Noise a signal nothing accepts.
func pendingSignalModel(t *testing.T) *exploreModel {
	t.Helper()
	return parseLibraryModel(t, `
		package test {
			private import ScalarValues::*;

			attribute def Stop;
			attribute def Go;
			attribute def Noise;

			state Machine {
				attribute log : String = "";

				entry; then top;
				state top {
					do { assign log := log + "did "; }
				}
				transition first top accept Stop do assign log := log + "stop " then idle;
				transition first top accept Go do assign log := log + "go " then idle;
				state idle;
			}
		}
	`)
}

// A signal in flight is delivered behind the events already queued, so the dispatch
// drawn against a due do step is the one the queue then dispatches: a queued event
// nothing accepts hides the signal until the round closes, a queued event that acts
// is drawn by name, and the signal is drawn by name only once nothing is ahead of it.
func TestStepOrderDrawsThePendingSignalAsQueued(t *testing.T) {
	m := pendingSignalModel(t)
	cases := []struct {
		name    string
		queued  string
		finals  []string
		choices []string
	}{
		{
			name:   "a queued event nothing accepts goes first, undrawn",
			queued: "Noise",
			finals: []string{`finalState idle; visits top, idle; log = "did stop "`},
		},
		{
			name:    "a queued event that acts is drawn, not the signal behind it",
			queued:  "Go",
			finals:  []string{`finalState idle; visits top, idle; log = "did go "`, `finalState idle; visits top, idle; log = "go "`},
			choices: []string{"do top", "dispatch accept Go"},
		},
		{
			name:    "the signal alone is drawn by name",
			finals:  []string{`finalState idle; visits top, idle; log = "did stop "`, `finalState idle; visits top, idle; log = "stop "`},
			choices: []string{"do top", "dispatch accept Stop"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			machine := m.state(t, "Machine")
			start := func(ctx *Context) (*Invocation, error) {
				exec, err := ctx.CreateStateExecutor(machine)
				if err != nil {
					return nil, err
				}
				if c.queued != "" {
					exec.SendSignal(c.queued, nil)
				}
				ctx.PostMessage(Message{SignalType: "Stop"})
				return &Invocation{States: []*StateExecutor{exec}, Horizon: HorizonAt(1)}, nil
			}
			report, err := Check(context.Background(), m.fresh, start, CheckBudget{}, unreduced(), nil)
			if err != nil {
				t.Fatal(err)
			}
			if report.Verdict == CheckViolation || len(report.BoundsHit) != 0 || len(report.NotEnumerated) != 0 {
				t.Fatalf("check: %s, want a complete search", report.Status())
			}
			if got := finalOutcomes(report); !slices.Equal(got, c.finals) {
				t.Fatalf("finals %v, want %v", got, c.finals)
			}
			for _, final := range report.Finals {
				steps := slices.DeleteFunc(slices.Clone(final.Witness.Choices), func(ch ChoiceTaken) bool {
					return ch.Kind != ChoiceStepOrder
				})
				switch {
				case c.choices == nil && len(steps) != 0:
					t.Fatalf("%s: step orders %v, want none while the queued event is ahead", final.Outcome, steps)
				case c.choices != nil && (len(steps) != 1 || !slices.Equal(steps[0].Among, c.choices)):
					t.Fatalf("%s: step orders %v, want one among %v", final.Outcome, steps, c.choices)
				}
				r := replayWitness(t, m, start, final.Witness, final.Outcome)
				if got := r.Ctx.Trace().String(); got != final.Witness.Trace {
					t.Errorf("replay of %s left the trace\n%s\nwant the witness's\n%s", final.Outcome, got, final.Witness.Trace)
				}
			}
		})
	}
}
