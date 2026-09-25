package solve

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeOptimizeQuery is the smallest optimizing query: one integer variable, one
// objective over exactly that variable, so a fake solver's single `get-value`
// reply answers both the model and the objective's own value.
func fakeOptimizeQuery(t *testing.T) *Query {
	t.Helper()
	ctx, idx := fixture(t, "fake_objective.sysml", `
		package test {
			private import ScalarValues::*;
			private import TradeStudies::*;
			analysis def Small {
				attribute size : Integer;
				require constraint { size >= 4 }
				objective smallest : MinimizeObjective {
					subject :>> selectedAlternative;
					in calc :>> eval { size }
				}
			}
		}`)
	sym := symbolNamed(t, idx, "test::Small")
	q, err := Analysis(ctx, sym, sym.OwnerScope)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	return q
}

// optimizingFake is the fake solver answering an optimizing dialogue: the
// scenario decides its verdicts, the reply its reported optima, the model the
// values it says attain them.
func optimizingFake(t *testing.T, scenario, objectives, model string) *Solver {
	t.Helper()
	solver := fakeSolver(t, scenario)
	solver.Env = append(solver.Env,
		objectivesEnv+"="+objectives,
		"OPENSYSML_TEST_SOLVER_MODEL="+model)
	return solver
}

// TestOptimumClassification: each form a backend reports an optimum in reaches
// the caller as what it establishes, and nothing more. The verdicts a scenario
// gives the verification checks decide whether a reported value is an optimum.
func TestOptimumClassification(t *testing.T) {
	cases := []struct {
		name       string
		scenario   string
		objectives string
		model      string
		want       OptimumStatus
		value      string
		bound      string
		says       string
	}{
		{
			name: "attained", scenario: "optimal",
			objectives: "(objectives (|test::Small::size| 4))", model: "((|test::Small::size| 4))",
			want: OptimumAttained, value: "4",
		},
		{
			name: "rational value", scenario: "optimal",
			objectives: "(objectives (|test::Small::size| (/ 9.0 2.0)))", model: "((|test::Small::size| (/ 9.0 2.0)))",
			want: OptimumAttained, value: "4.5",
		},
		{
			name: "unbounded", scenario: "optimal",
			objectives: "(objectives (|test::Small::size| (- oo)))", model: "((|test::Small::size| 4))",
			want: OptimumUnbounded, says: "arbitrarily smaller",
		},
		{
			name: "infinity against the direction", scenario: "optimal",
			objectives: "(objectives (|test::Small::size| oo))", model: "((|test::Small::size| 4))",
			want: OptimumUndecided, says: "no number",
		},
		{
			name: "infinitesimal bound", scenario: "optimal",
			objectives: "(objectives (|test::Small::size| (+ 4.0 (* 1.0 epsilon))))",
			model:      "((|test::Small::size| 5))",
			want:       OptimumBounded, bound: "4", says: "without any assignment attaining it",
		},
		{
			name: "interval bound", scenario: "optimal",
			objectives: "(objectives (|test::Small::size| (interval 4.0 6.0)))",
			model:      "((|test::Small::size| 5))",
			want:       OptimumBounded, bound: "4",
		},
		{
			name: "value no assignment attains", scenario: "optimal",
			objectives: "(objectives (|test::Small::size| 4))", model: "((|test::Small::size| 6))",
			want: OptimumBounded, bound: "4", says: "no assignment reported attains",
		},
		{
			name: "unreadable", scenario: "optimal",
			objectives: `(objectives (|test::Small::size| "best"))`, model: "((|test::Small::size| 4))",
			want: OptimumUndecided, says: "no number",
		},
		{
			// Every optimum is verified here rather than taken on the backend's
			// word, which is what catches a solver reporting a wrong one.
			name: "refuted by verification", scenario: "sat",
			objectives: "(objectives (|test::Small::size| 4))", model: "((|test::Small::size| 4))",
			want: OptimumUnverified, says: "a strictly smaller value is feasible",
		},
		{
			name: "verification undecided", scenario: "optimal-unknown",
			objectives: "(objectives (|test::Small::size| 4))", model: "((|test::Small::size| 4))",
			want: OptimumUndecided, says: "did not decide",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := fakeOptimizeQuery(t)
			solver := optimizingFake(t, tc.scenario, tc.objectives, tc.model)
			result, err := solver.Optimize(context.Background(), q)
			if err != nil {
				t.Fatalf("optimize: %v", err)
			}
			if result.Status != StatusSat || len(result.Optima) != 1 {
				t.Fatalf("result is %s with %d optima", result.Status, len(result.Optima))
			}
			got := result.Optima[0]
			if got.Status != tc.want {
				t.Errorf("optimum is %s (%q), want %s", got.Status, got.Detail, tc.want)
			}
			if got.Value != tc.value {
				t.Errorf("value is %q, want %q", got.Value, tc.value)
			}
			if got.Bound != tc.bound {
				t.Errorf("bound is %q, want %q", got.Bound, tc.bound)
			}
			if tc.says != "" && !strings.Contains(got.Detail, tc.says) {
				t.Errorf("detail is %q, want it to say %q", got.Detail, tc.says)
			}
			// A witness the conditions permit is reported whatever came of the
			// optimum, and is never presented as one.
			if got.Feasible == "" {
				t.Error("no feasible value reported")
			}
			if got.Status != OptimumAttained && got.Value != "" {
				t.Errorf("a value is reported as the optimum: %q", got.Value)
			}
		})
	}
}

// TestOptimizeKeepsVerdictsApart: an unsatisfiable or undecided query answers as
// itself, with no optima invented for it.
func TestOptimizeKeepsVerdictsApart(t *testing.T) {
	cases := []struct {
		scenario string
		want     Status
		reason   string
	}{
		{"unsat", StatusUnsat, ""},
		{"unknown", StatusUnknown, "incomplete arithmetic"},
	}
	for _, tc := range cases {
		t.Run(tc.scenario, func(t *testing.T) {
			solver := optimizingFake(t, tc.scenario, "(objectives (|test::Small::size| 4))", "")
			result, err := solver.Optimize(context.Background(), fakeOptimizeQuery(t))
			if err != nil {
				t.Fatalf("optimize: %v", err)
			}
			if result.Status != tc.want {
				t.Errorf("result is %s, want %s", result.Status, tc.want)
			}
			if len(result.Optima) != 0 {
				t.Errorf("reported optima for a %s query: %+v", tc.want, result.Optima)
			}
			if result.Reason != tc.reason {
				t.Errorf("reason is %q, want %q", result.Reason, tc.reason)
			}
		})
	}
}

// TestOptimizeRefusesABackendWithoutOptimization: optimization is a z3 extension,
// so a backend that does not implement it is reported rather than asked and
// misread, and never degraded to a plain satisfiability check.
func TestOptimizeRefusesABackendWithoutOptimization(t *testing.T) {
	cases := []struct {
		name     string
		solver   func(t *testing.T) *Solver
		says     string
		unwanted string
	}{
		{
			name: "declared not to implement it",
			solver: func(t *testing.T) *Solver {
				s := optimizingFake(t, "optimal", "(objectives (|test::Small::size| 4))", "")
				s.Name = "cvc5"
				// What a caller knowing its backend declares, probing nothing.
				s.Declared = DeclaredCapabilities("cvc5", CapModels, CapIncremental)
				return s
			},
			says: "declared not to support this",
		},
		{
			name: "rejects the optimization commands",
			solver: func(t *testing.T) *Solver {
				return optimizingFake(t, "optimal", `(error "unknown command minimize")`, "")
			},
			says: "rejected (get-objectives)",
		},
		{
			name: "answers something else",
			solver: func(t *testing.T) *Solver {
				return optimizingFake(t, "optimal", "unsupported", "")
			},
			says: "answered `unsupported`",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.solver(t).Optimize(context.Background(), fakeOptimizeQuery(t))
			if err == nil {
				t.Fatalf("a backend without optimization answered: %+v", result)
			}
			if !errors.Is(err, ErrNoOptimization) {
				t.Errorf("error is %v, want one saying the backend does not optimize", err)
			}
			if !strings.Contains(err.Error(), tc.says) {
				t.Errorf("error %q does not say %q", err, tc.says)
			}
		})
	}
}

// TestOptimizeReportsANonOptimizationRefusalAsItself: a capability the query needs
// anyway keeps its own report rather than being relabelled as no optimization.
func TestOptimizeReportsANonOptimizationRefusalAsItself(t *testing.T) {
	q := fakeOptimizeQuery(t)
	q.Vars = append(q.Vars, &Var{Name: "label", Sort: String})
	solver := optimizingFake(t, "optimal", "(objectives (|test::Small::size| 4))", "")
	solver.Declared = DeclaredCapabilities("fake", CapOptimization, CapOptimizationPriority)
	_, err := solver.Optimize(context.Background(), q)
	if !Unsupported(err) {
		t.Fatalf("error is %v, want a capability refusal", err)
	}
	if errors.Is(err, ErrNoOptimization) {
		t.Errorf("a refused string sort is reported as no optimization: %v", err)
	}
	var unsupported *UnsupportedCapabilityError
	if errors.As(err, &unsupported) && unsupported.Missing[0] != CapStrings {
		t.Errorf("refusal is about %v, want the strings capability", unsupported.Missing)
	}
}

// TestOptimizeRefusesABackendWithoutTheOptimizationCapability: a probed refusal of
// `(maximize …)` stops the query with the typed refusal, not a process failure.
func TestOptimizeRefusesABackendWithoutTheOptimizationCapability(t *testing.T) {
	solver := capabilitySolver(t, "maximize", "minimize")
	_, err := solver.Optimize(context.Background(), fakeOptimizeQuery(t))
	if err == nil {
		t.Fatal("a backend refusing optimization reported an optimum")
	}
	if !errors.Is(err, ErrNoOptimization) || !Unsupported(err) {
		t.Errorf("error is %v, want a refusal of the optimization capability", err)
	}
	if errors.Is(err, ErrSolverProcess) {
		t.Errorf("a refusal is reported as a process failure: %v", err)
	}
	var unsupported *UnsupportedCapabilityError
	if !errors.As(err, &unsupported) {
		t.Fatalf("error is %T, want one wrapping *UnsupportedCapabilityError", err)
	}
	if unsupported.Operation != "optimizing" || unsupported.Missing[0] != CapOptimization {
		t.Errorf("refusal is about %s %v, want optimizing and the optimization capability",
			unsupported.Operation, unsupported.Missing)
	}
	for _, want := range []string{"fake", "unsupported", "install z3"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not mention %q", err, want)
		}
	}
}

// TestOptimizeRejectsUnreadableOptima: a backend answering sat but not reporting
// its objectives readably is a failure, not an optimum.
func TestOptimizeRejectsUnreadableOptima(t *testing.T) {
	cases := []struct {
		name       string
		objectives string
		says       string
	}{
		{"an optimum too few", "(objectives)", "reported 0 optima for 1 objectives"},
		{"one optimum too many",
			"(objectives (|test::Small::size| 4) (|test::Small::size| 5))",
			"reported 2 optima for 1 objectives"},
		{"an entry that is no pair", "(objectives 4)", "rather than an objective and its optimum"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			solver := optimizingFake(t, "optimal", tc.objectives, "((|test::Small::size| 4))")
			_, err := solver.Optimize(context.Background(), fakeOptimizeQuery(t))
			if err == nil {
				t.Fatal("an unreadable answer was read as an optimum")
			}
			if !errors.Is(err, ErrNoOptimum) {
				t.Errorf("error is %v, want one saying no optimum was reported", err)
			}
			if !strings.Contains(err.Error(), tc.says) {
				t.Errorf("error %q does not say %q", err, tc.says)
			}
		})
	}
}

// TestOptimizeProcessFailures: a solver that dies or falls silent while being
// optimized fails as it does while being asked a satisfiability question, and is
// not mistaken for one that does not optimize — which would send the user after
// a z3 they already have.
func TestOptimizeProcessFailures(t *testing.T) {
	for _, scenario := range []string{"crash", "silent"} {
		t.Run(scenario, func(t *testing.T) {
			solver := optimizingFake(t, scenario, "(objectives (|test::Small::size| 4))", "")
			_, err := solver.Optimize(context.Background(), fakeOptimizeQuery(t))
			if err == nil {
				t.Fatal("a failed solver reported an optimum")
			}
			if !errors.Is(err, ErrSolverProcess) {
				t.Errorf("error is %v, want a process failure", err)
			}
			if errors.Is(err, ErrNoOptimization) {
				t.Errorf("a failed solver is reported as one that does not optimize: %v", err)
			}
		})
	}
}

// TestOptimizeRefusesAQueryWithoutObjectives: optimizing asks about objectives,
// so a query stating none is a refusal rather than a satisfiability check.
func TestOptimizeRefusesAQueryWithoutObjectives(t *testing.T) {
	q := intQuery(t)
	solver := optimizingFake(t, "optimal", "(objectives)", "")
	_, err := solver.Optimize(context.Background(), q)
	if !errors.Is(err, ErrNoObjective) {
		t.Errorf("error is %v, want one saying the query states no objective", err)
	}
	if _, err := solver.Optimize(context.Background(), nil); err == nil {
		t.Error("optimized no query at all")
	}
}

// TestOptimizeCarriesTheObjectiveThrough: an optimum names the objective it is
// about, so a report can say what it answers.
func TestOptimizeCarriesTheObjectiveThrough(t *testing.T) {
	solver := optimizingFake(t, "optimal",
		"(objectives (|test::Small::size| 4))", "((|test::Small::size| 4))")
	result, err := solver.Optimize(context.Background(), fakeOptimizeQuery(t))
	if err != nil {
		t.Fatalf("optimize: %v", err)
	}
	got := result.Optima[0]
	if got.Objective.Name != "smallest" || got.Objective.Direction != Minimize {
		t.Errorf("optimum is about %s %q", got.Objective.Direction, got.Objective.Name)
	}
	if got.Raw != "4" {
		t.Errorf("the solver's own expression for the optimum is %q, want 4", got.Raw)
	}
	if len(result.Model) != 1 || result.Model[0].Value != "4" {
		t.Errorf("model is %+v, want the assignment attaining the optimum", result.Model)
	}
}

// TestNoObjectiveErrorMessage: the refusal names the element and what it lacks.
func TestNoObjectiveErrorMessage(t *testing.T) {
	err := &NoObjectiveError{Element: "MassBudget"}
	if !strings.Contains(err.Error(), "MassBudget") || !strings.Contains(err.Error(), "objective") {
		t.Errorf("message is %q", err.Error())
	}
	if !errors.Is(err, ErrNoObjective) {
		t.Error("the error is not a no-objective one")
	}
}

// TestObjectiveErrorMessage: a refusal to translate an objective names the
// objective, why it was refused and where it was written.
func TestObjectiveErrorMessage(t *testing.T) {
	err := &ObjectiveError{
		Element:   "analysis Gain",
		Objective: "objective best",
		Reason:    "states a nonlinear value",
		Remedy:    "an optimizer improves a linear objective",
		Location:  "objectives.sysml:12:3",
	}
	for _, want := range []string{"analysis Gain", "objective best", "nonlinear",
		"objectives.sysml:12:3", "linear objective"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message %q does not say %q", err.Error(), want)
		}
	}
	if !errors.Is(err, ErrNotOptimizable) {
		t.Error("the error is not an untranslatable-objective one")
	}
}
