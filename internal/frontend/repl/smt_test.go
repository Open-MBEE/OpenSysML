package repl

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/smt"
	"github.com/Open-MBEE/OpenSysML/internal/exec/solve"
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

// An input the model leaves unbound frees the inputs without a release: under
// %engine all, check refuses the question instead of failing on the value it does
// not have, and smt's proof over the input's domain stands as the verdict.
func TestEngineAllLeavesUnboundInputsToSMT(t *testing.T) {
	s := symbolicSession(t, `
package Gate {
	private import ScalarValues::*;
	action def open {
		attribute n : Natural;
		attribute limit : Integer = 5;
		requirement natural { require constraint { n >= 0 } }
		first start;
		action add { assign limit := limit + 1; }
		done;
		succession first start then add;
		succession first add then done;
	}
}
`)
	run(t, s, "%engine all")
	run(t, s, "%check-property Gate::open::natural")
	v := s.RunAction("Gate::open")
	wantVerdict(t, v, VerdictHolds,
		"✓ Action Gate::open: holds",
		"inputs: n : Natural free in >= 0, limit = 5",
		"standing: holds (proved over schedules and inputs: inputs free in their domains: n : Natural free in >= 0); all: check refused (check cannot leave the inputs free), smt holds (proved)")
	if len(v.Plan.Disagreements) != 0 {
		t.Errorf("disagreements %+v, want none: check answered nothing", v.Plan.Disagreements)
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

// %check-bounds timeout is the solver's clock as well as the plan's: without it each query
// runs under the solver's own timeout, with it under the check's, as the solver bound names.
func TestEngineSMTRunsUnderTheCheckTimeout(t *testing.T) {
	s := symbolicSession(t, gateSource)
	own, err := solve.Discover()
	if err != nil {
		t.Fatal(err)
	}
	run(t, s, "%engine smt")
	run(t, s, "%check-property Gate::open::positive")
	if bound := planBound(t, s.RunAction("Gate::open"), "solver"); bound.Limit != own.Timeout.Milliseconds() {
		t.Errorf("solver bound without a timeout = %d ms, want the solver's own %s", bound.Limit, own.Timeout)
	}

	wants(t, run(t, s, "%check-bounds timeout=90s"), "timeout=1m30s")
	v := s.RunAction("Gate::open")
	wantVerdict(t, v, VerdictHolds, "standing: holds (proved over schedules: inputs as written)")
	if bound := planBound(t, v, "solver"); bound.Limit != (90 * time.Second).Milliseconds() {
		t.Errorf("solver bound under timeout=90s = %d ms, want 90000", bound.Limit)
	}
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
	run(t, s, "%check-bounds states=3")
	wantVerdict(t, s.RunAction("Gate::open"), VerdictUnresolved,
		"%check-bounds states is the check engine's, which %engine smt leaves out; select it, as %engine check, or every engine, as %engine all")
	run(t, s, "%check-diverge n")
	wantVerdict(t, s.RunAction("Gate::open"), VerdictUnresolved,
		"%check-bounds states is the check engine's, which %engine smt leaves out")
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

// raceSource: two branches write one feature, so its final value is the schedule's.
const raceSource = `
package Debug {
	private import ScalarValues::*;
	action race {
		attribute x : Integer = 0;
		attribute y : Integer = 0;
		first start;
		fork split;
		action left { assign x := 1; }
		action right { assign x := 2; }
		join sync;
		done;
		succession first start then split;
		succession first split then left;
		succession first split then right;
		succession first left then sync;
		succession first right then sync;
		succession first sync then done;
	}
}
`

// Under %engine smt the feature %check-diverge names is decided by the two-copy
// query: sensitive with a witness for each value, either of which %replay steps to
// the value it records; a feature the schedules agree on is proved not sensitive.
func TestEngineSMTDecidesSensitivity(t *testing.T) {
	s := symbolicSession(t, raceSource)
	dir := t.TempDir()
	run(t, s, "%engine smt")
	run(t, s, "%check-witness "+dir)
	wants(t, run(t, s, "%check-diverge x"), "check-diverge: x")
	fileA, fileB := filepath.Join(dir, "Debug.race-x-A.witness"), filepath.Join(dir, "Debug.race-x-B.witness")
	wantVerdict(t, s.RunAction("Debug::race"), VerdictFails,
		"✗ Action Debug::race: sensitive: x ends as 1 or 2; the schedules part at step 3:",
		"witness A: "+fileA, "witness B: "+fileB,
		"standing: sensitive (witnessed: witness of 1 choice replayed, inputs as written)")

	run(t, s, "%check-diverge y")
	wantVerdict(t, s.RunAction("Debug::race"), VerdictHolds,
		"✓ Action Debug::race: holds", "standing: holds (proved over schedules: inputs as written)")

	run(t, s, "%engine auto")
	values := map[string]bool{}
	for _, file := range []string{fileA, fileB} {
		wants(t, run(t, s, "%replay "+file), "schedule: replay:"+file)
		run(t, s, "%action Debug::race")
		out := run(t, s, "%continue")
		wants(t, out, "Action completed")
		for _, value := range []string{"x = 1", "x = 2"} {
			if strings.Contains(out, value) {
				values[value] = true
			}
		}
	}
	if len(values) != 2 {
		t.Errorf("the two witnesses replay to %v, want both values", values)
	}
}
