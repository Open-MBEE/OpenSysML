package engines

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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

// The build's registry is the framework's own engines and smt, in name order, and
// stays open to more.
func TestDefaultHoldsTheFrameworksEnginesAndSMT(t *testing.T) {
	r := Default()
	want := strings.Join([]string{analysis.CheckEngineName, analysis.ExploreEngineName, analysis.RunEngineName, analysis.SMTEngineName, analysis.SolveEngineName, analysis.SweepEngineName}, ", ")
	if got := names(r); got != want {
		t.Fatalf("engines %s, want %s", got, want)
	}
	if got := names(analysis.Default()); strings.Contains(got, analysis.SMTEngineName) {
		t.Fatalf("the framework's own registry holds smt: %s", got)
	}
	if err := r.Register(analysis.NewSolve(nil)); !errors.Is(err, analysis.ErrDuplicateEngine) {
		t.Fatalf("registering solve twice: %v, want the duplicate refused", err)
	}
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

// A holds question under auto reaches smt first, at proved, then check at bounded;
// under all both land in the plan.
func TestHoldsRanksSMTOverCheck(t *testing.T) {
	q := analysis.Question{Kind: analysis.Holds, Free: analysis.FreeSchedule, Holds: &analysis.HoldsAsk{}}
	plan, err := Default().Answer(context.Background(), &analysis.Model{}, q, analysis.Budget{})
	if err != nil {
		t.Fatalf("holds under auto: %v", err)
	}
	if len(plan.Steps) != 2 || plan.Steps[0].Engine != analysis.SMTEngineName || plan.Steps[1].Engine != analysis.CheckEngineName {
		t.Fatalf("steps %+v, want smt refusing the malformed question, then check", plan.Steps)
	}
	for _, step := range plan.Steps {
		if !errors.Is(step.Refusal, analysis.ErrMalformedQuestion) {
			t.Errorf("%s refused with %v, want the question malformed", step.Engine, step.Refusal)
		}
	}
	if plan.Result.Strength != analysis.NotCovered {
		t.Fatalf("result %s, want not covered", plan.Result.Strength)
	}
}

// The environment's tools join the build's registry as they join the framework's.
func TestDefaultFromEnvAddsTheManifestsTools(t *testing.T) {
	dir := t.TempDir()
	entry := analysis.ToolEntry{ToolName: "Zed", Version: "9", Executable: filepath.Join(dir, "zed"), Variables: []string{"x"}}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "zed"+analysis.ManifestExt), data, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(analysis.ToolsEnv, dir)
	r, err := DefaultFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if got := names(r); got != names(Default())+", tool:Zed" {
		t.Fatalf("engines %s, want the build's and tool:Zed", got)
	}
	t.Setenv(analysis.ToolsEnv, filepath.Join(dir, "none"))
	if _, err := DefaultFromEnv(); !errors.Is(err, analysis.ErrManifest) {
		t.Fatalf("DefaultFromEnv over a missing manifest: %v, want the manifest's fault", err)
	}
}
