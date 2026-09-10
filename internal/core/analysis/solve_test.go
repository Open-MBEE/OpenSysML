package analysis

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
)

// requireSolver returns the discovered solver, skipping the test when none is
// installed unless OPENSYSML_REQUIRE_SMT declares one mandatory.
func requireSolver(t *testing.T) *solve.Solver {
	t.Helper()
	solver, err := solve.Discover()
	if err == nil {
		return solver
	}
	if !errors.Is(err, solve.ErrNoSolver) {
		t.Fatalf("discover a solver: %v", err)
	}
	if os.Getenv("OPENSYSML_REQUIRE_SMT") != "" {
		t.Fatalf("OPENSYSML_REQUIRE_SMT is set but %v", err)
	}
	t.Skipf("no SMT solver installed: %v", err)
	return nil
}

// intQuery is a constraint on one integer, i in (low, high).
func intQuery(element string, low, high int64) *solve.Query {
	i := &solve.Var{Name: "test::" + element + "::i", Sort: solve.Int, Location: "c.sysml:3:3"}
	return &solve.Query{
		Kind:    "constraint",
		Element: element,
		Vars:    []*solve.Var{i},
		Assertions: []solve.Assertion{
			{Term: solve.Binary(solve.OpGt, solve.Bool, solve.VarTerm(i), solve.IntTerm(low)), From: solve.Provenance{Kind: "constraint", Element: element, Condition: "i > low", Role: solve.RoleRequired}},
			{Term: solve.Binary(solve.OpLt, solve.Bool, solve.VarTerm(i), solve.IntTerm(high)), From: solve.Provenance{Kind: "constraint", Element: element, Condition: "i < high", Role: solve.RoleRequired}},
		},
	}
}

// satisfiable asks the default registry about the queries with the solver's Solve.
func satisfiable(t *testing.T, queries ...*solve.Query) Plan {
	t.Helper()
	q := Question{Kind: Satisfiable, Subject: "test::C", Free: FreeInputs, Solve: &SolveAsk{Queries: queries, Ask: (*solve.Solver).Solve}}
	return answered(t, Default(), nil, q, Budget{})
}

func TestSolveProvesUnsatisfiable(t *testing.T) {
	requireSolver(t)
	result := satisfiable(t, intQuery("C", 5, 2)).Result
	if result.Engine != SolveEngineName || result.Claim != ClaimUnsatisfiable || result.Strength != Proved {
		t.Fatalf("result %+v, want solve's proof of unsatisfiability", result)
	}
	if len(result.Values) != 1 || result.Values[0].Solved == nil || result.Values[0].Solved.Status != solve.StatusUnsat || result.Values[0].Name != "C" {
		t.Fatalf("values %+v, want C's unsat verdict", result.Values)
	}
	if limit, ok := result.Bounds.Limit("solver"); !ok || limit != solve.DefaultTimeout.Milliseconds() || result.Bounds.Reached() {
		t.Fatalf("bounds %s, want the default solver time unreached", result.Bounds)
	}
}

func TestSolveWitnessesSatisfiable(t *testing.T) {
	requireSolver(t)
	result := satisfiable(t, intQuery("C", 2, 5)).Result
	if result.Claim != ClaimSatisfiable || result.Strength != Witnessed {
		t.Fatalf("result %+v, want a witnessed satisfiable", result)
	}
	solved := result.Values[0].Solved
	if solved.Status != solve.StatusSat || len(solved.Model) != 1 || (solved.Model[0].Value != "3" && solved.Model[0].Value != "4") {
		t.Fatalf("solved %+v, want a replayed witness in (2, 5)", solved)
	}
}

func TestSolveJudgesTheSetOfQueries(t *testing.T) {
	requireSolver(t)
	result := satisfiable(t, intQuery("A", 2, 5), intQuery("B", 5, 2)).Result
	if result.Claim != ClaimUnsatisfiable || result.Strength != Proved || len(result.Values) != 2 {
		t.Fatalf("result %+v, want one unsatisfiable query deciding the set", result)
	}
	if result.Values[0].Solved.Status != solve.StatusSat || result.Values[1].Solved.Status != solve.StatusUnsat {
		t.Fatalf("values %+v, want A sat and B unsat in order", result.Values)
	}
}

func TestRegistrySolveIsTheSolversAnswer(t *testing.T) {
	solver := requireSolver(t)
	values, err := Default().Solve(context.Background(), "test::C", []*solve.Query{intQuery("C", 2, 5)}, (*solve.Solver).Explain, Budget{Solver: time.Minute})
	if err != nil {
		t.Fatalf("solve: %v", err)
	}
	direct, err := solver.Explain(context.Background(), intQuery("C", 2, 5))
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	if len(values) != 1 || values[0].Err != nil || values[0].Solved.Status != direct.Status || values[0].Solved.Solver != direct.Solver {
		t.Fatalf("values %+v, want the solver's own %+v", values, direct)
	}
	if _, err = registered(t, NewSolve(absentSolver)).Solve(context.Background(), "test::C", []*solve.Query{intQuery("C", 2, 5)}, (*solve.Solver).Solve, Budget{}); !errors.Is(err, solve.ErrNoSolver) {
		t.Fatalf("solve without a solver: %v, want its absence", err)
	}
}

func TestSolveGradesEachVerdict(t *testing.T) {
	exact := intQuery("C", 2, 5)
	rounded := intQuery("R", 2, 5)
	rounded.Assertions = append(rounded.Assertions, solve.Assertion{Term: solve.Binary(solve.OpGt, solve.Bool, solve.ToReal(solve.VarTerm(rounded.Vars[0])), solve.IntTerm(0))})
	if !rounded.Rounded() {
		t.Fatal("the widened query must round")
	}
	objective := solve.Objective{Name: "m", Direction: solve.Minimize, Term: solve.VarTerm(exact.Vars[0])}
	cases := []struct {
		name     string
		value    Evaluation
		claim    Claim
		strength Strength
		reason   string
	}{
		{"unsat", Evaluation{Name: "C", Solved: &solve.Result{Query: exact, Status: solve.StatusUnsat}}, ClaimUnsatisfiable, Proved, ""},
		{"rounded unsat", Evaluation{Name: "R", Solved: &solve.Result{Query: rounded, Status: solve.StatusUnsat}}, ClaimNone, NotCovered, "R rounds in floating point when evaluated, which an exact-real unsat does not decide"},
		{"sat", Evaluation{Name: "C", Solved: &solve.Result{Query: exact, Status: solve.StatusSat}}, ClaimSatisfiable, Witnessed, ""},
		{"unknown", Evaluation{Name: "C", Solved: &solve.Result{Query: exact, Status: solve.StatusUnknown, Reason: "timeout"}}, ClaimNone, NotCovered, "timeout"},
		{"silent unknown", Evaluation{Name: "C", Solved: &solve.Result{Query: exact, Status: solve.StatusUnknown}}, ClaimNone, NotCovered, "the solver did not decide C"},
		{"failed", Evaluation{Name: "C", Err: errors.New("the solver exited")}, ClaimNone, NotCovered, "the solver exited"},
		{"not asked", Evaluation{Name: "C"}, ClaimNone, NotCovered, "C was not put to the solver"},
		{"attained", Evaluation{Name: "C", Solved: &solve.Result{Query: exact, Status: solve.StatusSat, Optima: []solve.Optimum{{Objective: objective, Status: solve.OptimumAttained, Value: "3"}}}}, ClaimSatisfiable, Witnessed, ""},
		{"unbounded", Evaluation{Name: "C", Solved: &solve.Result{Query: exact, Status: solve.StatusSat, Optima: []solve.Optimum{{Objective: objective, Status: solve.OptimumUnbounded}}}}, ClaimUnbounded, Proved, ""},
		{"bounded", Evaluation{Name: "C", Solved: &solve.Result{Query: exact, Status: solve.StatusSat, Optima: []solve.Optimum{{Objective: objective, Status: solve.OptimumBounded, Detail: "approaches 2"}}}}, ClaimNone, NotCovered, "m: approaches 2"},
		{"unverified", Evaluation{Name: "C", Solved: &solve.Result{Query: exact, Status: solve.StatusSat, Optima: []solve.Optimum{{Objective: objective, Status: solve.OptimumUnverified, Detail: "a better value is feasible"}}}}, ClaimNone, NotCovered, "m: a better value is feasible"},
		{"undecided", Evaluation{Name: "C", Solved: &solve.Result{Query: exact, Status: solve.StatusSat, Optima: []solve.Optimum{{Objective: objective, Status: solve.OptimumUndecided, Detail: "unknown"}}}}, ClaimNone, NotCovered, "m: unknown"},
	}
	for _, tc := range cases {
		claim, strength, reason := judgeOne(tc.value)
		if claim != tc.claim || strength != tc.strength || reason != tc.reason {
			t.Errorf("%s: %s %s %q, want %s %s %q", tc.name, claim, strength, reason, tc.claim, tc.strength, tc.reason)
		}
	}
}

func TestSolveJudgesSetsAsTheirWeakestMember(t *testing.T) {
	exact := intQuery("C", 2, 5)
	sat := Evaluation{Name: "A", Solved: &solve.Result{Query: exact, Status: solve.StatusSat}}
	unsat := Evaluation{Name: "B", Solved: &solve.Result{Query: exact, Status: solve.StatusUnsat}}
	unknown := Evaluation{Name: "U", Solved: &solve.Result{Query: exact, Status: solve.StatusUnknown, Reason: "gave up"}}
	unbounded := Evaluation{Name: "O", Solved: &solve.Result{Query: exact, Status: solve.StatusSat, Optima: []solve.Optimum{{Status: solve.OptimumUnbounded}}}}
	cases := []struct {
		name     string
		values   []Evaluation
		claim    Claim
		strength Strength
		reason   string
	}{
		{"all sat", []Evaluation{sat, sat}, ClaimSatisfiable, Witnessed, ""},
		{"one unsat", []Evaluation{sat, unsat}, ClaimUnsatisfiable, Proved, ""},
		{"unknown after unsat", []Evaluation{unsat, unknown}, ClaimNone, NotCovered, "gave up"},
		{"unbounded among sat", []Evaluation{sat, unbounded}, ClaimUnbounded, Proved, ""},
		{"unsat over unbounded", []Evaluation{unbounded, unsat}, ClaimUnsatisfiable, Proved, ""},
		{"none", nil, ClaimSatisfiable, Witnessed, ""},
		{"withheld", []Evaluation{sat, {Name: "W"}}, ClaimNone, NotCovered, "W was not put to the solver"},
	}
	for _, tc := range cases {
		claim, strength, reason := judgeSolved(tc.values, len(tc.values))
		if claim != tc.claim || strength != tc.strength || reason != tc.reason {
			t.Errorf("%s: %s %s %q, want %s %s %q", tc.name, claim, strength, reason, tc.claim, tc.strength, tc.reason)
		}
	}
}

func TestSolveTakesTheBudgetsSolverTime(t *testing.T) {
	requireSolver(t)
	q := Question{Kind: Satisfiable, Subject: "test::C", Free: FreeInputs, Solve: &SolveAsk{Queries: []*solve.Query{intQuery("C", 2, 5)}, Ask: (*solve.Solver).Solve}}
	result := answered(t, Default(), nil, q, Budget{Solver: 2 * time.Second}).Result
	if limit, ok := result.Bounds.Limit("solver"); !ok || limit != 2000 {
		t.Fatalf("bounds %s, want the budget's 2s of solver time", result.Bounds)
	}
	timedOut := []Evaluation{{Name: "C", Solved: &solve.Result{Status: solve.StatusUnknown, TimedOut: true}}}
	if !anyTimedOut(timedOut) || anyTimedOut(nil) {
		t.Fatal("a timed-out verdict must reach the solver bound")
	}
}

// The budget's runs are the queries solve asks: the rest are left unasked, which the
// set's verdict and the runs bound both report.
func TestSolveAsksNoMoreQueriesThanTheBudgetsRuns(t *testing.T) {
	present := func() (*solve.Solver, error) { return &solve.Solver{Name: "z3", Path: "/usr/bin/z3"}, nil }
	var asked []string
	sat := func(_ *solve.Solver, _ context.Context, query *solve.Query) (*solve.Result, error) {
		asked = append(asked, query.Element)
		return &solve.Result{Query: query, Status: solve.StatusSat}, nil
	}
	queries := []*solve.Query{intQuery("A", 2, 5), intQuery("B", 2, 5), intQuery("C", 2, 5)}
	q := Question{Kind: Satisfiable, Subject: "test::C", Free: FreeInputs, Solve: &SolveAsk{Queries: queries, Ask: sat}}
	result := answered(t, registered(t, NewSolve(present)), nil, q, Budget{Runs: 2}).Result
	if len(asked) != 2 || asked[0] != "A" || asked[1] != "B" {
		t.Fatalf("asked %v, want A and B only", asked)
	}
	if result.Claim != ClaimNone || result.Strength != NotCovered || result.Reason != "C was left unasked by the runs budget" {
		t.Fatalf("result %+v, want not covered for the unasked C", result)
	}
	if len(result.Values) != 3 || result.Values[2].Solved != nil || result.Values[2].Err != nil || result.Values[2].Name != "C" {
		t.Fatalf("values %+v, want C's unasked entry in place", result.Values)
	}
	if runs, ok := result.Bounds.Limit("runs"); !ok || runs != 2 || !result.Bounds.Reached() {
		t.Fatalf("bounds %s, want the budget's 2 runs reached", result.Bounds)
	}
	asked = nil
	result = answered(t, registered(t, NewSolve(present)), nil, q, Budget{Runs: 3}).Result
	if len(asked) != 3 || result.Claim != ClaimSatisfiable || result.Bounds.Reached() {
		t.Fatalf("asked %v, result %+v; want every query asked within 3 runs", asked, result)
	}
	result = answered(t, registered(t, NewSolve(present)), nil, q, Budget{}).Result
	if runs, ok := result.Bounds.Limit("runs"); !ok || runs != 3 || result.Bounds.Reached() || result.Claim != ClaimSatisfiable {
		t.Fatalf("result %+v, want every query asked and the 3 named as the runs without a runs budget", result)
	}
	withheld := func(_ *solve.Solver, _ context.Context, query *solve.Query) (*solve.Result, error) {
		if query.Element == "B" {
			return nil, nil
		}
		return &solve.Result{Query: query, Status: solve.StatusSat}, nil
	}
	q.Solve = &SolveAsk{Queries: queries, Ask: withheld}
	result = answered(t, registered(t, NewSolve(present)), nil, q, Budget{Runs: 3}).Result
	if result.Strength != NotCovered || result.Reason != "B was not put to the solver" || result.Bounds.Reached() {
		t.Fatalf("result %+v, want the withheld B not covered without blaming the runs budget", result)
	}
}

// The bounds solve declares are the bounds its result reports, in that order.
func TestSolveReportsTheBoundsItDeclares(t *testing.T) {
	present := func() (*solve.Solver, error) { return &solve.Solver{Name: "z3", Path: "/usr/bin/z3"}, nil }
	sat := func(_ *solve.Solver, _ context.Context, query *solve.Query) (*solve.Result, error) {
		return &solve.Result{Query: query, Status: solve.StatusSat}, nil
	}
	engine := NewSolve(present)
	q := Question{Kind: Satisfiable, Subject: "test::A", Free: FreeInputs, Solve: &SolveAsk{Queries: []*solve.Query{intQuery("A", 2, 5), intQuery("B", 2, 5)}, Ask: sat}}
	declared := engine.Describe().Bounds
	for _, budget := range []Budget{{}, {Runs: 1}, {Runs: 5}} {
		result := answered(t, registered(t, engine), nil, q, budget).Result
		if len(result.Bounds) != len(declared) {
			t.Fatalf("%+v: bounds %s, want the declared %v", budget, result.Bounds, declared)
		}
		for i, bound := range result.Bounds {
			if bound.Name != declared[i] {
				t.Errorf("%+v: bound %d is %q, want the declared %q", budget, i, bound.Name, declared[i])
			}
		}
	}
}

func TestSolveRefusesWhatItCannotAsk(t *testing.T) {
	e := NewSolve(func() (*solve.Solver, error) { return &solve.Solver{Name: "z3", Path: "/usr/bin/z3"}, nil })
	ask := &SolveAsk{Queries: []*solve.Query{intQuery("C", 2, 5)}, Ask: (*solve.Solver).Solve}
	if c := e.Covers(nil, Question{Kind: Holds}); c.Covered || !errors.Is(c.Refusal, ErrNotAsked) {
		t.Fatalf("holds: %+v, want not asked", c)
	}
	if c := e.Covers(nil, Question{Kind: Satisfiable, Free: FreeInputs | FreeSchedule, Solve: ask}); c.Covered || !errors.Is(c.Refusal, ErrFreedom) {
		t.Fatalf("free schedule: %+v, want a freedom refusal", c)
	}
	if c := e.Covers(nil, Question{Kind: Satisfiable, Free: FreeInputs}); c.Covered || !errors.Is(c.Refusal, ErrMalformedQuestion) {
		t.Fatalf("no ask: %+v, want malformed", c)
	}
	if c := e.Covers(nil, Question{Kind: Satisfiable, Free: FreeInputs, Solve: &SolveAsk{Ask: ask.Ask}}); c.Covered || !errors.Is(c.Refusal, ErrMalformedQuestion) {
		t.Fatalf("no queries: %+v, want malformed", c)
	}
	if c := e.Covers(nil, Question{Kind: Satisfiable, Free: FreeInputs, Solve: ask}); !c.Covered {
		t.Fatalf("satisfiable: %+v, want covered", c)
	}
}
