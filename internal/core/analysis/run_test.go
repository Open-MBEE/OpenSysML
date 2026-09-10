package analysis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// evaluate asks the default registry for one execution in the fixture's context.
func evaluate(t *testing.T, f *fixture, perform Performance) Plan {
	t.Helper()
	q := Question{Kind: Evaluate, Subject: "test::Double", Schedule: runtime.DefaultSchedulePolicy, Perform: perform}
	return answered(t, Default(), Held(f.context(t), nil), q, Budget{})
}

func TestRunObservesAValue(t *testing.T) {
	f := parseFixture(t)
	double := f.symbol(t, "Double")
	plan := evaluate(t, f, func(ctx *runtime.Context) (Answer, error) {
		value, err := ctx.InvokeCalc(double, []runtime.Value{intOf(21)}, f.pkg)
		return ValuesAnswer([]Evaluation{{Name: "result", Value: value}}, err), nil
	})
	result := plan.Result
	if result.Engine != RunEngineName || result.Claim != ClaimValue || result.Strength != Observed {
		t.Fatalf("result %+v, want run's observed value", result)
	}
	if len(result.Values) != 1 || result.Values[0].Value.Const.Int != 42 {
		t.Fatalf("values %+v, want result = 42", result.Values)
	}
	if steps, ok := result.Bounds.Limit("steps"); !ok || steps != 10000 || result.Bounds.Reached() {
		t.Fatalf("bounds %s, want steps=10000 unreached", result.Bounds)
	}
	if result.Witness != nil {
		t.Fatalf("witness %+v, want none for a value", result.Witness)
	}
}

func TestRunGradesChecks(t *testing.T) {
	f := parseFixture(t)
	holds := evaluate(t, f, func(*runtime.Context) (Answer, error) {
		return CheckAnswer(runtime.CheckResult{Holds: true}, nil), nil
	}).Result
	if holds.Claim != ClaimHolds || holds.Strength != Observed || holds.Witness != nil {
		t.Fatalf("holding check %+v, want holds observed", holds)
	}
	violated := evaluate(t, f, func(*runtime.Context) (Answer, error) {
		return CheckAnswer(runtime.CheckResult{Holds: false}, nil), nil
	}).Result
	if violated.Claim != ClaimViolated || violated.Strength != Witnessed {
		t.Fatalf("violated check %+v, want violated witnessed", violated)
	}
	if violated.Witness == nil || violated.Witness.Schedule != runtime.DefaultSchedulePolicy {
		t.Fatalf("witness %+v, want the run's own schedule", violated.Witness)
	}
}

// A false verdict reaches CheckAnswer as a *runtime.ViolationError, which is a
// witnessed violation and not a failed execution.
func TestRunWitnessesARuntimeViolation(t *testing.T) {
	f := parseFixture(t)
	tank := f.symbol(t, "Tank")
	rctx := f.context(t)
	inst, err := rctx.Instantiate(tank)
	if err != nil {
		t.Fatalf("instantiate Tank: %v", err)
	}
	check := func(name string) Plan {
		sym, ok := tank.Scope.LookupLocal(name)
		if !ok {
			t.Fatalf("constraint %s not indexed", name)
		}
		return answered(t, Default(), Held(rctx, nil), Question{
			Kind: Evaluate, Subject: "test::Tank::" + name, Schedule: runtime.DefaultSchedulePolicy,
			Perform: func(rctx *runtime.Context) (Answer, error) {
				return CheckAnswer(rctx.CheckConstraintOn(sym, tank.Scope, inst)), nil
			},
		}, Budget{})
	}
	low := check("low").Result
	if low.Claim != ClaimHolds || low.Strength != Observed {
		t.Fatalf("low %+v, want holds observed", low)
	}
	high := check("high").Result
	if high.Claim != ClaimViolated || high.Strength != Witnessed || high.Witness == nil {
		t.Fatalf("high %+v, want violated witnessed", high)
	}
	if !strings.Contains(high.Reason, "pressure > 100.0") {
		t.Fatalf("reason %q, want the failed condition", high.Reason)
	}
	if high.Bounds.Reached() {
		t.Fatalf("bounds %s, want no budget reached by a violation", high.Bounds)
	}

	wrapped := fmt.Errorf("check: %w", &runtime.ViolationError{Kind: "constraint", Element: "high", What: "assertion", Condition: "pressure > 100.0"})
	if a := CheckAnswer(runtime.CheckResult{}, wrapped); a.Claim != ClaimViolated || a.Err != nil || a.Reason != wrapped.Error() {
		t.Fatalf("wrapped violation %+v, want violated with the violation as reason", a)
	}
}

func TestRunFailureClaimsNothingAndNamesTheBound(t *testing.T) {
	f := parseFixture(t)
	failed := fmt.Errorf("spin: %w", runtime.ErrActionStepLimitExceeded)
	plan := evaluate(t, f, func(*runtime.Context) (Answer, error) {
		return ValuesAnswer(nil, failed), nil
	})
	result := plan.Result
	if result.Covered() || result.Claim != ClaimNone || result.Reason != failed.Error() {
		t.Fatalf("result %+v, want not covered with the failure as reason", result)
	}
	if !result.Bounds.Reached() {
		t.Fatalf("bounds %s, want steps reached", result.Bounds)
	}
	for _, b := range result.Bounds {
		if b.Reached != (b.Name == "steps") {
			t.Fatalf("bounds %s, want steps alone reached", result.Bounds)
		}
	}
	if len(plan.Steps) != 1 || plan.Steps[0].Result == nil {
		t.Fatalf("steps %v, want run's result kept", stepNames(plan))
	}
}

func TestRunFaultStopsThePlan(t *testing.T) {
	f := parseFixture(t)
	fault := errors.New("the subject did not resolve")
	q := Question{Kind: Evaluate, Subject: "test::Missing", Schedule: runtime.DefaultSchedulePolicy, Perform: func(*runtime.Context) (Answer, error) {
		return Answer{}, fault
	}}
	plan, err := Default().Answer(context.Background(), Held(f.context(t), nil), q, Budget{})
	if !errors.Is(err, fault) || len(plan.Steps) != 1 || !errors.Is(plan.Steps[0].Err, fault) {
		t.Fatalf("answer: %v, steps %+v; want the fault stopping the plan", err, plan.Steps)
	}
}

func TestRunRefusesWhatItCannotFix(t *testing.T) {
	e := NewRun()
	if c := e.Covers(nil, Question{Kind: Outcomes}); c.Covered || !errors.Is(c.Refusal, ErrNotAsked) {
		t.Fatalf("outcomes: %+v, want not asked", c)
	}
	if c := e.Covers(nil, Question{Kind: Evaluate, Free: FreeSchedule, Perform: func(*runtime.Context) (Answer, error) { return Answer{}, nil }}); c.Covered || !errors.Is(c.Refusal, ErrFreedom) {
		t.Fatalf("free schedule: %+v, want a freedom refusal", c)
	}
	if c := e.Covers(nil, Question{Kind: Evaluate}); c.Covered || !errors.Is(c.Refusal, ErrMalformedQuestion) {
		t.Fatalf("no perform: %+v, want malformed", c)
	}
}

func TestPerformReturnsWhatTheCallProduced(t *testing.T) {
	f := parseFixture(t)
	double := f.symbol(t, "Double")
	ctx := f.context(t)
	value, _, err := Perform(context.Background(), Default(), Held(ctx, nil), "test::Double", ctx.Schedule(), Budget{}, Auto(),
		func(rt *runtime.Context) (runtime.Value, error) {
			return rt.InvokeCalc(double, []runtime.Value{intOf(4)}, f.pkg)
		},
		func(v runtime.Value, err error) Answer {
			return ValuesAnswer([]Evaluation{{Name: "result", Value: v}}, err)
		},
	)
	if err != nil || value.Const.Int != 8 {
		t.Fatalf("perform: %v = %+v, want 8", err, value)
	}
	failed := errors.New("unbound parameter")
	_, _, err = Perform(context.Background(), Default(), Held(ctx, nil), "test::Double", ctx.Schedule(), Budget{}, Auto(),
		func(*runtime.Context) (runtime.Value, error) { return runtime.Value{}, failed },
		func(v runtime.Value, err error) Answer { return ValuesAnswer(nil, err) },
	)
	if !errors.Is(err, failed) {
		t.Fatalf("perform: %v, want the execution's own failure back", err)
	}
}

func TestPerformReportsARefusal(t *testing.T) {
	f := parseFixture(t)
	ctx := f.context(t)
	r := registered(t, NewExplore())
	_, _, err := Perform(context.Background(), r, Held(ctx, nil), "test::Double", ctx.Schedule(), Budget{}, Auto(),
		func(*runtime.Context) (int, error) { return 1, nil },
		func(int, error) Answer { return Answer{Claim: ClaimValue} },
	)
	if !errors.Is(err, ErrNoEngine) {
		t.Fatalf("perform without run: %v, want no engine", err)
	}
	r = registered(t, fakeEngine{name: "picky", kinds: []Kind{Evaluate}, authority: Observed, refusal: errFixtureRefusal})
	_, _, err = Perform(context.Background(), r, Held(ctx, nil), "test::Double", ctx.Schedule(), Budget{}, Auto(),
		func(*runtime.Context) (int, error) { return 1, nil },
		func(int, error) Answer { return Answer{Claim: ClaimValue} },
	)
	if !errors.Is(err, errFixtureRefusal) {
		t.Fatalf("perform on a refusing engine: %v, want its refusal", err)
	}
}

func TestAnswersOfRuns(t *testing.T) {
	failed := errors.New("failed")
	if a := VerificationAnswer(runtime.VerificationResult{Verdict: runtime.VerificationVerdict{Kind: runtime.VerdictPass}}, nil); a.Claim != ClaimHolds {
		t.Fatalf("pass %+v, want holds", a)
	}
	if a := VerificationAnswer(runtime.VerificationResult{Verdict: runtime.VerificationVerdict{Kind: runtime.VerdictFail, Detail: "x > 1"}}, nil); a.Claim != ClaimViolated || a.Reason != "x > 1" {
		t.Fatalf("fail %+v, want violated with the detail", a)
	}
	if a := VerificationAnswer(runtime.VerificationResult{Verdict: runtime.VerificationVerdict{Kind: runtime.VerdictInconclusive}}, nil); a.Claim != ClaimNone {
		t.Fatalf("inconclusive %+v, want nothing claimed", a)
	}
	if a := VerificationAnswer(runtime.VerificationResult{Verdict: runtime.VerificationVerdict{Kind: runtime.VerdictPass}}, failed); a.Claim != ClaimNone || a.Err != failed {
		t.Fatalf("erroring %+v, want nothing claimed", a)
	}
	outputs := []runtime.CalcOutputValue{{Name: "m", Value: intOf(1)}}
	if a := CaseAnswer(runtime.AnalysisResult{Outputs: outputs}, nil); a.Claim != ClaimValue || len(a.Values) != 1 || a.Values[0].Name != "m" {
		t.Fatalf("case %+v, want its outputs as values", a)
	}
	if a := CaseAnswer(runtime.AnalysisResult{Outputs: outputs}, failed); a.Claim != ClaimNone || len(a.Values) != 1 {
		t.Fatalf("failed case %+v, want the outputs kept beside the failure", a)
	}
	values := ValuesOf(map[string]runtime.Value{"z": intOf(3), "a": intOf(1)})
	if len(values) != 2 || values[0].Name != "a" || values[1].Name != "z" {
		t.Fatalf("held values %+v, want name order", values)
	}
	if a := CheckAnswer(runtime.CheckResult{}, failed); a.Claim != ClaimNone || a.Err != failed {
		t.Fatalf("failed check %+v, want nothing claimed", a)
	}
}

// A caller already gone when the run is asked for is answered with its own error
// before the execution starts, the run engine's step carrying it.
func TestRunStopsWhenTheCallerIsGone(t *testing.T) {
	f := parseFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	performed := false
	q := Question{Kind: Evaluate, Subject: "test::Double", Schedule: runtime.DefaultSchedulePolicy,
		Perform: func(*runtime.Context) (Answer, error) {
			performed = true
			return Answer{Claim: ClaimValue}, nil
		}}
	plan, err := Default().Answer(ctx, Held(f.context(t), nil), q, Budget{})
	if !errors.Is(err, context.Canceled) || performed {
		t.Fatalf("answer for a gone caller: %v, performed %v; want context.Canceled unperformed", err, performed)
	}
	if len(plan.Steps) != 1 || plan.Steps[0].Engine != RunEngineName ||
		!errors.Is(plan.Steps[0].Err, context.Canceled) {
		t.Fatalf("steps %+v, want run's step carrying the cancellation", plan.Steps)
	}
	if plan.Result.Covered() {
		t.Fatalf("result %+v, want nothing established", plan.Result)
	}
}
