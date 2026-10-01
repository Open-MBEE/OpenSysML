package runtime

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// loopingDoModel loads the conformance case whose `looping` state runs a `do`
// body of two branches each waiting on the clock and looping back, while a
// timed transition leaves the state; nested puts the machine on a part two deep.
func loopingDoModel(t *testing.T, nested bool) *exploreModel {
	t.Helper()
	more := ""
	if nested {
		more = `
	part def Vehicle { exhibit state modes : Machine; }
	part def Mission { part vehicle : Vehicle; }
	part mission : Mission;
`
	}
	return parseLibraryModel(t, loopingDoText(t, more))
}

// loopingDoText is the conformance case's text with more members in its package.
func loopingDoText(t *testing.T, more string) string {
	t.Helper()
	text, err := os.ReadFile(filepath.Join("testdata", "conformance", "state_do_action_loop_timed_exit.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(text[:len(text)-2]) + more + "}\n"
}

// nestedStateStarterOf starts the machine on the object the path reaches under an
// instance of the part, up to the horizon.
func nestedStateStarterOf(part, sym *symbols.Symbol, path string, horizon Horizon) Starter {
	return func(ctx *Context) (*Invocation, error) {
		if _, err := ctx.Instantiate(part); err != nil {
			return nil, err
		}
		obj, err := ctx.objectAt(path)
		if err != nil {
			return nil, err
		}
		exec, err := ctx.CreateStateExecutorFor(sym, obj)
		if err != nil {
			return nil, err
		}
		return &Invocation{States: []*StateExecutor{exec}, Horizon: horizon}, nil
	}
}

// Every witness of a check over a state whose `do` body loops through timed
// waits replays to its trace: at the round the body's branches are due together
// the timed exit is due too, and the moves the checker records there — the step
// order between the body and the exit after each token move, and the token order
// — are the moves replay makes, whether the machine runs at top level or on a part
// nested in another. The check diverges over `left` and `right`: the exit before
// either branch wrote, after one, or after both.
func TestCheckWitnessesOfALoopingDoRoundReplay(t *testing.T) {
	for _, nested := range []bool{false, true} {
		name := "top-level"
		if nested {
			name = "nested performer"
		}
		t.Run(name, func(t *testing.T) {
			m := loopingDoModel(t, nested)
			sym := m.state(t, "Machine")
			start := stateStarterOf(sym, HorizonAt(5))
			if nested {
				mission := namedOrFoundSymbol(t, m.idx, "test::mission", m.idx.DocumentRoot(m.path), ast.DefPart, ast.UsagePart)
				start = nestedStateStarterOf(mission, sym, "test::mission#1.vehicle", HorizonAt(5))
			}
			report, err := Check(context.Background(), m.fresh, start, CheckBudget{}, unreduced(), nil)
			if err != nil {
				t.Fatal(err)
			}
			if report.Verdict != CheckDivergent || len(report.Finals) == 0 || len(report.Violations) != 0 {
				t.Fatalf("check: %s, want no violation, divergent, with finals", report.Status())
			}
			if len(report.BoundsHit) != 0 {
				t.Fatalf("check: %s, want no bound hit", report.Status())
			}
			features := make([]string, 0, len(report.Divergent))
			for _, d := range report.Divergent {
				features = append(features, d.Feature)
			}
			if !slices.Equal(features, []string{"left", "right"}) {
				t.Fatalf("divergent %v, want left and right: the exit drawn before, between or after the branches' moves", report.Divergent)
			}
			for _, final := range report.Finals {
				if final.Values["finalState"] != "heard+finished" || final.Values["late"] != "1" {
					t.Fatalf("final %s, want heard+finished with late at 1", final.Outcome)
				}
				if !slices.ContainsFunc(final.Witness.Choices, func(c ChoiceTaken) bool { return c.Kind == ChoiceStepOrder }) {
					t.Fatalf("witness %s records no step order at the round the exit is due", FormatChoices(final.Witness.Choices))
				}
				if !slices.ContainsFunc(final.Witness.Choices, func(c ChoiceTaken) bool { return c.Kind == ChoiceTokenOrder }) {
					t.Fatalf("witness %s records no token order at the do round", FormatChoices(final.Witness.Choices))
				}
				r := replayWitness(t, m, start, final.Witness, final.Outcome)
				if got := r.Ctx.Trace().String(); got != final.Witness.Trace {
					t.Errorf("replay of %s left the trace\n%s\nwant the witness's\n%s", final.Outcome, got, final.Witness.Trace)
				}
			}
		})
	}
}

// An action performed inline in another's flow steps its own tokens within the
// performer's step: the checker records the inner branches' order at the step
// number the performer's token has, and replay follows it in the inner flow
// rather than refusing it against the outer token.
func TestCheckWitnessesOfAnInlinePerformanceReplay(t *testing.T) {
	m := parseExploreModel(t, inlinePerformanceModel)
	sym := m.action(t, "outer")
	start := starterOf(sym)
	report, err := Check(context.Background(), m.fresh, start, CheckBudget{}, unreduced(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != CheckExhaustive || len(report.Finals) != 1 {
		t.Fatalf("check: %s, want no violation, exhaustive, one final", report.Status())
	}
	for _, f := range report.Finals {
		steps := make([]int, 0, len(f.Witness.Choices))
		for _, c := range f.Witness.Choices {
			steps = append(steps, c.Step)
		}
		if !slices.Equal(steps, []int{3}) {
			t.Fatalf("witness %s, want the inner order alone, at the performer's step 3", FormatChoices(f.Witness.Choices))
		}
		r := replayWitness(t, m, start, f.Witness, f.Outcome)
		if got := r.Ctx.Trace().String(); got != f.Witness.Trace {
			t.Errorf("replay left the trace\n%s\nwant the witness's\n%s", got, f.Witness.Trace)
		}
	}
	run := func(ctx *Context) (Outcome, error) {
		outputs, err := ctx.ExecuteAction(sym)
		if err != nil {
			return Outcome{}, err
		}
		return ctx.ActionOutcome(outputs), nil
	}
	x := m.exploreAction(t, "explore", "outer")
	if !x.Complete() || len(x.Outcomes) != 1 {
		t.Fatalf("explore: %s, want one outcome, complete", x.Status())
	}
	assertWitnessesReplay(t, x, m.fresh, run)
}
