package analysis

import (
	"context"
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
)

// fakeEngine is an engine scripted for one test: what it covers, what it
// answers, and the fault it reports.
type fakeEngine struct {
	name      string
	kinds     []Kind
	authority Strength
	refusal   error
	result    Result
	fault     error
	ran       *int
}

func (e fakeEngine) Name() string { return e.name }

func (e fakeEngine) Describe() Description {
	return Description{Questions: e.kinds, Authority: e.authority}
}

func (e fakeEngine) Covers(*Model, Question) Coverage {
	if e.refusal != nil {
		return refused(e.refusal)
	}
	return covered
}

func (e fakeEngine) Run(_ context.Context, _ *Model, q Question, _ Budget) (Result, error) {
	if e.ran != nil {
		*e.ran++
	}
	if e.fault != nil {
		return Result{}, e.fault
	}
	result := e.result
	result.Question, result.Engine = q, e.name
	return result, nil
}

// registered builds a registry of the engines, failing the test on a refusal.
func registered(t *testing.T, engines ...Engine) *Registry {
	t.Helper()
	r := NewRegistry()
	for _, e := range engines {
		if err := r.Register(e); err != nil {
			t.Fatalf("register %s: %v", e.Name(), err)
		}
	}
	return r
}

// names lists the registry's engines as Engines orders them.
func names(r *Registry) []string {
	var out []string
	for _, e := range r.Engines() {
		out = append(out, e.Name())
	}
	return out
}

func TestRegisterDuplicateIsATypedError(t *testing.T) {
	r := registered(t, fakeEngine{name: "one"})
	err := r.Register(fakeEngine{name: "one"})
	var dup *DuplicateEngineError
	if !errors.As(err, &dup) || dup.Name != "one" || !errors.Is(err, ErrDuplicateEngine) {
		t.Fatalf("second registration of one: %v, want DuplicateEngineError naming it", err)
	}
	if got := names(r); len(got) != 1 {
		t.Fatalf("engines %v, want the first registration alone", got)
	}
}

func TestEnginesAreInNameOrder(t *testing.T) {
	r := registered(t, fakeEngine{name: "solve"}, fakeEngine{name: "explore"}, fakeEngine{name: "run"}, fakeEngine{name: "check"})
	want := []string{"check", "explore", "run", "solve"}
	for i, got := range names(r) {
		if got != want[i] {
			t.Fatalf("engines %v, want %v", names(r), want)
		}
	}
	if len(names(r)) != len(want) {
		t.Fatalf("engines %v, want %v", names(r), want)
	}
}

func TestRegistriesDoNotSeeEachOther(t *testing.T) {
	first := registered(t, fakeEngine{name: "one"})
	second := registered(t, fakeEngine{name: "two"})
	if got := names(first); len(got) != 1 || got[0] != "one" {
		t.Fatalf("first registry %v, want [one]", got)
	}
	if got := names(second); len(got) != 1 || got[0] != "two" {
		t.Fatalf("second registry %v, want [two]", got)
	}
	if err := second.Register(fakeEngine{name: "one"}); err != nil {
		t.Fatalf("one in the second registry: %v, want no clash with the first", err)
	}
}

func TestDefaultHoldsTheBuildsEngines(t *testing.T) {
	want := []string{ExploreEngineName, RunEngineName, SolveEngineName, SweepEngineName}
	got := names(Default())
	if len(got) != len(want) {
		t.Fatalf("default engines %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("default engines %v, want %v", got, want)
		}
	}
	if err := Default().Register(fakeEngine{name: "one"}); err != nil {
		t.Fatalf("registering into a default registry: %v", err)
	}
}

// absentSolver discovers nothing, as a machine without a solver does.
func absentSolver() (*solve.Solver, error) {
	return nil, &solve.NoSolverError{Looked: []string{"z3", "cvc5"}}
}

func TestAbsentProcessListsAndRefuses(t *testing.T) {
	r := registered(t, NewRun(), NewSolve(absentSolver))
	statuses := r.Statuses()
	if len(statuses) != 2 || statuses[0].Engine != RunEngineName || statuses[1].Engine != SolveEngineName {
		t.Fatalf("statuses %v, want run then solve", statuses)
	}
	if statuses[0].Err != nil || statuses[0].Process != "" {
		t.Fatalf("run status %+v, want an in-process engine", statuses[0])
	}
	var absent *ProcessAbsentError
	if !errors.As(statuses[1].Err, &absent) || !errors.Is(statuses[1].Err, ErrProcessAbsent) || !errors.Is(statuses[1].Err, solve.ErrNoSolver) {
		t.Fatalf("solve status %v, want ProcessAbsentError wrapping the solver's absence", statuses[1].Err)
	}
	if absent.Engine != SolveEngineName || absent.Process != SolveProcess {
		t.Fatalf("absence %+v, want solve's need named", absent)
	}
	q := Question{Kind: Satisfiable, Subject: "C", Free: FreeInputs, Solve: &SolveAsk{Queries: []*solve.Query{{Element: "C"}}, Ask: (*solve.Solver).Solve}}
	coverage := r.Engines()[1].Covers(nil, q)
	if coverage.Covered || !errors.Is(coverage.Refusal, ErrProcessAbsent) {
		t.Fatalf("covers %+v, want a ProcessAbsentError refusal", coverage)
	}
	plan := answered(t, r, nil, q, Budget{})
	if err := plan.Refused(); !errors.Is(err, ErrProcessAbsent) {
		t.Fatalf("plan refused %v, want the solver's absence", err)
	}
	if plan.Result.Covered() || plan.Result.Reason == "" {
		t.Fatalf("result %+v, want not covered with the refusal as reason", plan.Result)
	}
}

func TestPresentProcessIsListed(t *testing.T) {
	present := func() (*solve.Solver, error) { return &solve.Solver{Name: "z3", Path: "/usr/bin/z3"}, nil }
	statuses := registered(t, NewSolve(present)).Statuses()
	if len(statuses) != 1 || statuses[0].Err != nil || statuses[0].Process != "z3 at /usr/bin/z3" {
		t.Fatalf("statuses %+v, want z3 at /usr/bin/z3", statuses)
	}
}
