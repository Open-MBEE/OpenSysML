package repl

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/smt"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
)

// gateSource: the requirement holds at limit's default and fails for other
// values of it; the assumption admits only values where it holds.
const gateSource = `
package Gate {
	private import ScalarValues::*;
	action def open {
		attribute n : Integer = 1;
		attribute limit : Integer = 5;
		requirement positive { require constraint { n + limit > 0 } }
		constraint wide { limit >= 0 and limit < 100 }
		first start;
		action add { assign n := n + 1; }
		done;
		succession first start then add;
		succession first add then done;
	}
}
`

// symbolicSession is a session over the build's registry, which holds smt, or is
// skipped (failed under OPENSYSML_REQUIRE_SMT) without a solver.
func symbolicSession(t *testing.T, source string) *Session {
	t.Helper()
	if _, err := solve.Discover(); err != nil {
		if !errors.Is(err, solve.ErrNoSolver) {
			t.Fatalf("discover a solver: %v", err)
		}
		if os.Getenv("OPENSYSML_REQUIRE_SMT") != "" {
			t.Fatalf("OPENSYSML_REQUIRE_SMT is set but %v", err)
		}
		t.Skipf("no SMT solver installed: %v", err)
	}
	return loadSource(t, source)
}

// Under %engine smt the property is proved on the inputs as written, unrolled
// to the engine's own move bound; once %check-input releases the bound one, it
// is decided over every value of it, the witness names the value the solver
// chose and the file opens with it. Under %engine check the release is refused
// as smt's alone, and without it the same question holds on the inputs as
// written: each report says why.
func TestEngineSMTRangesOverAReleasedInput(t *testing.T) {
	s := symbolicSession(t, gateSource)
	dir := t.TempDir()
	wants(t, run(t, s, "%engine smt"), "engine: smt")
	run(t, s, "%check-property Gate::open::positive")
	run(t, s, "%check-witness "+dir)
	proved := s.RunAction("Gate::open")
	wantVerdict(t, proved, VerdictHolds,
		"✓ Action Gate::open: holds",
		"inputs: n = 1, limit = 5",
		"standing: holds (proved over schedules: inputs as written)")
	if moves := planBound(t, proved, "moves"); moves.Limit != smt.DefaultMoves {
		t.Errorf("moves bound under %%engine smt alone = %d, want the engine's default %d", moves.Limit, smt.DefaultMoves)
	}

	run(t, s, "%check-input limit")
	witness := filepath.Join(dir, "Gate.open.violation-1.witness")
	wantVerdict(t, s.RunAction("Gate::open"), VerdictFails,
		"✗ Action Gate::open: at step 0: requirement positive: require condition evaluated to false: n + limit > 0",
		"inputs: n = 1, limit = -",
		"witness: "+witness,
		"standing: violated (witnessed: witness of 1 input replayed, inputs chosen from their domains: limit = -")
	content, err := os.ReadFile(witness)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(content), "input limit = -") {
		t.Errorf("witness file does not open with the input the solver chose:\n%s", content)
	}

	run(t, s, "%engine check")
	wantVerdict(t, s.RunAction("Gate::open"), VerdictUnresolved, "%check-input is the smt engine's, which %engine check leaves out")
	run(t, s, "%check-input off")
	wantVerdict(t, s.RunAction("Gate::open"), VerdictHolds,
		"no violation, exhaustive",
		"standing: holds (bounded over schedules: 4 states, 3 moves searched)")
}

// An assumption over the initial state excludes the violating values, and the
// proof lists it beside the input it ranges over.
func TestEngineSMTAssumesOverTheInitialState(t *testing.T) {
	s := symbolicSession(t, gateSource)
	run(t, s, "%engine smt")
	run(t, s, "%check-property Gate::open::positive")
	run(t, s, "%check-input limit")
	run(t, s, "%check-assume Gate::open::wide")
	wantVerdict(t, s.RunAction("Gate::open"), VerdictHolds,
		"✓ Action Gate::open: holds",
		"inputs: n = 1, limit : Integer free",
		"assumed: constraint wide",
		"standing: holds (proved over schedules and inputs: inputs free in their domains: limit : Integer free, assumed constraint wide)")
}

// Under %engine all, releasing a bound input puts the question to smt with the
// input free and shows check refusing it in the plan beside smt's answer.
func TestEngineAllShowsCheckRefusingFreeInputs(t *testing.T) {
	s := symbolicSession(t, gateSource)
	run(t, s, "%engine all")
	run(t, s, "%check-property Gate::open::positive")
	run(t, s, "%check-input limit")
	v := s.RunAction("Gate::open")
	wantVerdict(t, v, VerdictFails,
		"inputs: n = 1, limit = -",
		"all: check refused (check cannot leave the inputs free), smt violated (witnessed)")
	var refused, answered bool
	for _, step := range v.Plan.Steps {
		switch step.Engine {
		case analysis.CheckEngineName:
			refused = errors.Is(step.Refusal, analysis.ErrFreedom)
		case analysis.SMTEngineName:
			answered = step.Result != nil && step.Result.Claim == analysis.ClaimViolated
		}
	}
	if !refused || !answered {
		t.Errorf("plan steps %+v, want check refusing the free inputs and smt answering", v.Plan.Steps)
	}
}

// planBound is the named bound of the one result the verdict's plan answered with.
func planBound(t *testing.T, v Verdict, name string) analysis.Bound {
	t.Helper()
	if v.Plan == nil {
		t.Fatalf("verdict carries no plan:\n%s", strings.Join(v.Lines, "\n"))
	}
	for _, step := range v.Plan.Steps {
		if step.Result == nil {
			continue
		}
		for _, b := range step.Result.Bounds {
			if b.Name == name {
				return b
			}
		}
	}
	t.Fatalf("no %s bound in the plan %+v", name, v.Plan.Steps)
	return analysis.Bound{}
}

// A released name that is not a feature of the action is refused naming it.
func TestEngineSMTRefusesAnUnknownInput(t *testing.T) {
	s := symbolicSession(t, gateSource)
	run(t, s, "%engine smt")
	run(t, s, "%check-property Gate::open::positive")
	run(t, s, "%check-input nothing")
	wantVerdict(t, s.RunAction("Gate::open"), VerdictUnresolved, "nothing", "no such feature")
}

// A setting the smt engine alone reads, with no property named, puts the action to
// smt under %engine all: a released input or an assumption beside check's
// refusal, the unroll bound beside check's own answer.
func TestEngineAllPutsSMTOnlySettingsToSMT(t *testing.T) {
	for _, tc := range []struct {
		name, setting, plan string
	}{
		{"input", "%check-input limit", "all: check refused (check cannot leave the inputs free), smt holds (proved)"},
		{"assume", "%check-assume Gate::open::wide", "all: check refused (check cannot leave the inputs free), smt holds (proved)"},
		{"unroll", "%check-bounds unroll=2", "all: check holds (bounded), smt holds (proved)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := symbolicSession(t, gateSource)
			run(t, s, "%engine all")
			run(t, s, tc.setting)
			v := s.RunAction("Gate::open")
			wantVerdict(t, v, VerdictHolds, "✓ Action Gate::open: holds", tc.plan)
			for _, step := range v.Plan.Steps {
				if step.Engine == analysis.ExploreEngineName {
					t.Errorf("explore took part in a holds question: %+v", v.Plan.Steps)
				}
			}
		})
	}
}

// A setting one engine alone reads is refused under the other engine alone,
// naming it, rather than dropped from the question.
func TestEngineRefusesTheOtherEnginesSettings(t *testing.T) {
	s := loadSource(t, gateSource)
	run(t, s, "%engine smt")
	run(t, s, "%check-diverge n")
	wantVerdict(t, s.RunAction("Gate::open"), VerdictUnresolved,
		"%check-diverge is the check engine's, which %engine smt leaves out; select it, as %engine check, or every engine, as %engine all")
	run(t, s, "%check-bounds states=3")
	wantVerdict(t, s.RunAction("Gate::open"), VerdictUnresolved,
		"%check-diverge and %check-bounds states are the check engine's, which %engine smt leaves out")
	run(t, s, "%check-diverge off")
	run(t, s, "%check-bounds off")

	run(t, s, "%engine check")
	run(t, s, "%check-input limit")
	wantVerdict(t, s.RunAction("Gate::open"), VerdictUnresolved,
		"%check-input is the smt engine's, which %engine check leaves out; select it, as %engine smt, or every engine, as %engine all")
	run(t, s, "%check-assume Gate::open::wide")
	run(t, s, "%check-bounds unroll=2")
	wantVerdict(t, s.RunAction("Gate::open"), VerdictUnresolved,
		"%check-input, %check-assume and %check-bounds unroll are the smt engine's, which %engine check leaves out")
}
