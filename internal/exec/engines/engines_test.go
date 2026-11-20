package engines

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/solve"
)

func names(r *analysis.Registry) string {
	var out []string
	for _, e := range r.Engines() {
		out = append(out, e.Name())
	}
	return strings.Join(out, ", ")
}

// smt's status is its solver's: found and named, or the typed absence, as solve's is.
func TestSMTStatusIsTheSolvers(t *testing.T) {
	var smt, sol analysis.Status
	for _, s := range Default().Statuses() {
		switch s.Engine {
		case analysis.SMTEngineName:
			smt = s
		case analysis.SolveEngineName:
			sol = s
		}
	}
	if smt.Engine == "" || sol.Engine == "" {
		t.Fatal("smt or solve missing from the statuses")
	}
	if (smt.Err == nil) != (sol.Err == nil) {
		t.Fatalf("smt %+v and solve %+v disagree on the solver", smt, sol)
	}
	if smt.Err != nil {
		var absent *analysis.ProcessAbsentError
		if !errors.As(smt.Err, &absent) || !errors.Is(smt.Err, solve.ErrNoSolver) || absent.Engine != analysis.SMTEngineName {
			t.Fatalf("smt status %v, want ProcessAbsentError over the solver's absence", smt.Err)
		}
	} else if smt.Process == "" {
		t.Fatalf("smt status %+v, want the solver named", smt)
	}
}

// A holds question under auto reaches the proved engines first — smt, then
// solve — before check at bounded; under all they land in the plan.
func TestHoldsRanksSMTOverCheck(t *testing.T) {
	q := analysis.Question{Kind: analysis.Holds, Free: analysis.FreeSchedule, Holds: &analysis.HoldsAsk{}}
	plan, err := Default().Answer(context.Background(), &analysis.Model{}, q, analysis.Budget{})
	if err != nil {
		t.Fatalf("holds under auto: %v", err)
	}
	if len(plan.Steps) != 3 || plan.Steps[0].Engine != analysis.SMTEngineName || plan.Steps[1].Engine != analysis.SolveEngineName || plan.Steps[2].Engine != analysis.CheckEngineName {
		t.Fatalf("steps %+v, want smt and solve refusing the malformed question, then check", plan.Steps)
	}
	for i, step := range plan.Steps {
		want := analysis.ErrMalformedQuestion
		if step.Engine == analysis.SolveEngineName {
			want = analysis.ErrFreedom
		}
		if !errors.Is(step.Refusal, want) {
			t.Errorf("step %d %s refused with %v, want %v", i, step.Engine, step.Refusal, want)
		}
	}
	if plan.Result.Strength != analysis.NotCovered {
		t.Fatalf("result %s, want not covered", plan.Result.Strength)
	}
}
