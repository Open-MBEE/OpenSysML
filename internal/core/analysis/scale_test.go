package analysis

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
)

// everyClaim and everyStrength enumerate the scale's axes.
var (
	everyClaim    = []Claim{ClaimNone, ClaimHolds, ClaimViolated, ClaimSensitive, ClaimValue, ClaimTable, ClaimOutcomes, ClaimSatisfiable, ClaimUnsatisfiable, ClaimUnbounded}
	everyStrength = []Strength{NotCovered, Observed, Witnessed, Bounded, Proved}
)

// The scale admits exactly: nothing at not covered, an existential claim at witnessed, a
// concrete claim at observed, and a universal claim at any strength but witnessed.
func TestConsistentAdmitsTheScale(t *testing.T) {
	admitted := func(c Claim, s Strength) bool {
		switch {
		case c == ClaimNone:
			return s == NotCovered
		case c.Existential():
			return s == Witnessed
		case c.Concrete():
			return s == Observed
		}
		return s != NotCovered && s != Witnessed
	}
	for _, c := range everyClaim {
		for _, s := range everyStrength {
			err := Consistent(c, s)
			if (err == nil) != admitted(c, s) {
				t.Fatalf("consistent(%s, %s) = %v, want admitted %v", c, s, err, admitted(c, s))
			}
			if err != nil && !errors.Is(err, ErrInconsistentResult) {
				t.Fatalf("consistent(%s, %s) = %v, want the typed inconsistency", c, s, err)
			}
		}
	}
}

// scriptedSolver is a solve engine whose answers a test writes, in place of a process.
func scriptedSolver(t *testing.T, answer func(*solve.Query) *solve.Result) (*Registry, Question) {
	t.Helper()
	r := registered(t, NewSolve(func() (*solve.Solver, error) { return &solve.Solver{Name: "scripted", Path: "scripted"}, nil }))
	q := Question{Kind: Satisfiable, Subject: "test::C", Free: FreeInputs, Solve: &SolveAsk{
		Queries: []*solve.Query{intQuery("C", 2, 5)},
		Ask: func(_ *solve.Solver, _ context.Context, query *solve.Query) (*solve.Result, error) {
			return answer(query), nil
		},
	}}
	return r, q
}

// Every (Claim, Strength) pair an engine on develop produces, each driven for real, each
// consistent with the scale, and no universal claim stronger than its engine's authority.
func TestEveryEngineClaimStrengthPair(t *testing.T) {
	f := parseFixture(t)
	double := f.symbol(t, "Double")
	evaluation := func(perform Performance) func(t *testing.T) Plan {
		return func(t *testing.T) Plan { return evaluate(t, f, perform) }
	}
	exploration := func(budget Budget) func(t *testing.T) Plan {
		return func(t *testing.T) Plan {
			q := Question{Kind: Outcomes, Subject: "test::race", Schedule: policy(t, "explore"), Free: FreeSchedule, Linearize: raceRun(t, f)}
			return answered(t, Default(), &Model{Fresh: f.fresh}, q, budget)
		}
	}
	solving := func(status solve.Status, reason string, optima ...solve.Optimum) func(t *testing.T) Plan {
		return func(t *testing.T) Plan {
			r, q := scriptedSolver(t, func(query *solve.Query) *solve.Result {
				result := &solve.Result{Query: query, Status: status, Solver: "scripted", Reason: reason, Optima: optima}
				if status == solve.StatusSat {
					result.Model = []solve.Assignment{{Var: query.Vars[0], Value: "3", Raw: "3"}}
				}
				return result
			})
			return answered(t, r, nil, q, Budget{})
		}
	}
	for _, tc := range []struct {
		engine   string
		claim    Claim
		strength Strength
		standing string
		drive    func(t *testing.T) Plan
	}{
		{RunEngineName, ClaimValue, Observed, "value (observed: 1 run under reverse)", evaluation(func(ctx *runtime.Context) (Answer, error) {
			value, err := ctx.InvokeCalc(double, []runtime.Value{intOf(21)}, f.pkg)
			return ValuesAnswer([]Evaluation{{Name: "result", Value: value}}, err), nil
		})},
		{RunEngineName, ClaimHolds, Observed, "holds (observed: 1 run under reverse)", evaluation(func(*runtime.Context) (Answer, error) {
			return CheckAnswer(runtime.CheckResult{Holds: true}, nil), nil
		})},
		{RunEngineName, ClaimViolated, Witnessed, "violated (witnessed: 1 run under reverse)", evaluation(func(*runtime.Context) (Answer, error) {
			return CheckAnswer(runtime.CheckResult{Holds: false}, nil), nil
		})},
		{RunEngineName, ClaimNone, NotCovered, "not covered (evaluation step limit exceeded; steps=10000 (reached))", evaluation(func(*runtime.Context) (Answer, error) {
			return ValuesAnswer(nil, runtime.ErrStepLimitExceeded), nil
		})},
		{ExploreEngineName, ClaimOutcomes, Proved, "outcomes (proved over schedules: 6 linearizations, inputs as written)", exploration(Budget{})},
		{ExploreEngineName, ClaimOutcomes, Observed, "outcomes (observed: 1 linearization, inputs as written, runs=1 (reached))", exploration(Budget{Runs: 1})},
		{SweepEngineName, ClaimTable, Observed, "table (observed: 3 rows)", func(t *testing.T) Plan {
			ctx := f.context(t)
			q := Question{Kind: Sweep, Subject: "test::Double", Schedule: ctx.Schedule(), Sweep: &SweepAsk{Plan: doublePlan(t, f, ctx), Row: doubleRow(t, f, ctx)}}
			return answered(t, Default(), Held(ctx, nil), q, Budget{})
		}},
		{SolveEngineName, ClaimSatisfiable, Witnessed, "satisfiable (witnessed: 1 query by solve)", solving(solve.StatusSat, "")},
		{SolveEngineName, ClaimUnsatisfiable, Proved, "unsatisfiable (proved over inputs: 1 query by solve)", solving(solve.StatusUnsat, "")},
		{SolveEngineName, ClaimUnbounded, Proved, "unbounded (proved over inputs: 1 query by solve)", solving(solve.StatusSat, "", solve.Optimum{Status: solve.OptimumUnbounded})},
		{SolveEngineName, ClaimNone, NotCovered, "not covered (the solver gave up)", solving(solve.StatusUnknown, "the solver gave up")},
	} {
		t.Run(tc.engine+"/"+tc.claim.String()+"/"+tc.strength.String(), func(t *testing.T) {
			result := tc.drive(t).Result
			if result.Engine != tc.engine || result.Claim != tc.claim || result.Strength != tc.strength {
				t.Fatalf("result %+v, want %s answering %s (%s)", result, tc.engine, tc.claim, tc.strength)
			}
			if err := Consistent(result.Claim, result.Strength); err != nil {
				t.Fatalf("result %+v is off the scale: %v", result, err)
			}
			for _, e := range Default().Engines() {
				if e.Name() == tc.engine && result.Claim.Universal() && result.Strength > e.Describe().Authority {
					t.Fatalf("result %+v claims more than %s's authority %s", result, tc.engine, e.Describe().Authority)
				}
			}
			if result.Standing() != tc.standing {
				t.Fatalf("standing %q, want %q", result.Standing(), tc.standing)
			}
		})
	}
}

// A budget reached lowers the strength and the standing prints the bound: the same
// exploration is proved when it completes and observed under a budget of one run.
func TestABudgetReachedLowersTheStrengthAndPrintsTheBound(t *testing.T) {
	f := parseFixture(t)
	q := Question{Kind: Outcomes, Subject: "test::race", Schedule: policy(t, "explore"), Free: FreeSchedule, Linearize: raceRun(t, f)}
	complete := answered(t, Default(), &Model{Fresh: f.fresh}, q, Budget{}).Result
	cut := answered(t, Default(), &Model{Fresh: f.fresh}, q, Budget{Runs: 1}).Result
	if complete.Strength != Proved || complete.Bounds.Reached() {
		t.Fatalf("complete %+v, want proved with no bound reached", complete)
	}
	if cut.Strength != Observed || cut.Claim != ClaimOutcomes {
		t.Fatalf("cut %+v, want the same claim lowered to observed", cut)
	}
	if limit, ok := cut.Bounds.Limit("runs"); !ok || limit != 1 || !cut.Bounds.Reached() {
		t.Fatalf("bounds %s, want runs=1 reached", cut.Bounds)
	}
	if !strings.Contains(cut.Standing(), "observed") || !strings.Contains(cut.Standing(), "runs=1 (reached)") || strings.Contains(cut.Standing(), "proved") {
		t.Fatalf("standing %q, want the lowered strength and the bound", cut.Standing())
	}
	if strings.Contains(complete.Standing(), "reached") {
		t.Fatalf("standing %q, want no bound named when none was reached", complete.Standing())
	}
}

// A witness the evaluator rejects on replay is not covered with the disagreement as its
// reason — never violated or satisfiable — and under all it yields to a proof, never
// contradicting it.
func TestAWitnessThatFailsReplayIsNotCovered(t *testing.T) {
	const rejected = "the evaluator's floating-point arithmetic rejects the witness at the required `i > low`"
	r, q := scriptedSolver(t, func(query *solve.Query) *solve.Result {
		return &solve.Result{Query: query, Status: solve.StatusUnknown, Solver: "scripted", Reason: rejected}
	})
	result := answered(t, r, nil, q, Budget{}).Result
	if result.Covered() || result.Claim != ClaimNone || result.Reason != rejected {
		t.Fatalf("result %+v, want not covered with the replay's disagreement", result)
	}
	if err := r.Register(fakeEngine{name: "prover", kinds: []Kind{Satisfiable}, authority: Proved, result: Result{Claim: ClaimUnsatisfiable, Strength: Proved}}); err != nil {
		t.Fatalf("register: %v", err)
	}
	for _, selection := range []Selection{Auto(), All()} {
		plan, err := r.AnswerWith(context.Background(), nil, q, Budget{}, selection)
		if err != nil {
			t.Fatalf("%s: %v", selection, err)
		}
		if plan.Result.Claim != ClaimUnsatisfiable || plan.Result.Engine != "prover" || len(plan.Disagreements) != 0 {
			t.Fatalf("%s: result %+v (%d disagreements), want the proof to stand undisputed", selection, plan.Result, len(plan.Disagreements))
		}
		for _, step := range plan.Steps {
			if step.Result != nil && (step.Result.Claim == ClaimSatisfiable || step.Result.Claim == ClaimViolated) {
				t.Fatalf("%s: step %+v, want the failed replay never taken as a witness", selection, step)
			}
		}
	}
}

// No path promotes: the engines' own authorities cap what dispatch accepts, and
// composition never grades a set of results above the strongest of them.
func TestNoPathPromotesTheStrength(t *testing.T) {
	for _, tc := range []struct {
		authority, claimed Strength
	}{{Observed, Bounded}, {Observed, Proved}, {Bounded, Proved}} {
		r := registered(t, fakeEngine{name: "e", kinds: []Kind{Holds}, authority: tc.authority, result: Result{Claim: ClaimHolds, Strength: tc.claimed}})
		if _, err := r.Answer(context.Background(), nil, holdsQuestion, Budget{}); !errors.Is(err, ErrOverclaim) {
			t.Fatalf("%s engine claiming %s: %v, want the overclaim refused", tc.authority, tc.claimed, err)
		}
	}
	for _, s := range []Strength{Observed, Bounded} {
		results := []Result{
			{Question: holdsQuestion, Engine: "a", Claim: ClaimHolds, Strength: s},
			{Question: holdsQuestion, Engine: "b", Claim: ClaimHolds, Strength: s},
			{Question: holdsQuestion, Engine: "c", Claim: ClaimHolds, Strength: s},
		}
		if composed, _ := Compose(results); composed.Strength != s {
			t.Fatalf("three %s results composed to %s, want %s", s, composed.Strength, s)
		}
	}
}
