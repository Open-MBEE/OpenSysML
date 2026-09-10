package analysis

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

func TestParseSelectionReadsTheFlag(t *testing.T) {
	for text, want := range map[string]Selection{
		"":        Auto(),
		"auto":    Auto(),
		" auto ":  Auto(),
		"all":     All(),
		"explore": Only("explore"),
	} {
		if got := ParseSelection(text); got != want {
			t.Fatalf("parse %q: %+v, want %+v", text, got, want)
		}
	}
	for _, s := range []Selection{Auto(), All(), Only("solve")} {
		if ParseSelection(s.String()) != s {
			t.Fatalf("%+v does not round-trip through %q", s, s.String())
		}
	}
}

func TestSelectRefusesAnUnknownEngine(t *testing.T) {
	r := registered(t, fakeEngine{name: "b"}, fakeEngine{name: "a"})
	_, err := r.Select("c")
	var unknown *UnknownEngineError
	if !errors.As(err, &unknown) || !errors.Is(err, ErrUnknownEngine) || unknown.Name != "c" || !sameNames(unknown.Known, []string{"a", "b"}) {
		t.Fatalf("select c: %v, want the typed unknown-engine error naming a and b", err)
	}
	if !strings.Contains(err.Error(), `"c"`) || !strings.Contains(err.Error(), "a, b") {
		t.Fatalf("message %q, want the name and the choices", err)
	}
	for _, text := range []string{"a", "auto", "all", ""} {
		if _, err := r.Select(text); err != nil {
			t.Fatalf("select %q: %v", text, err)
		}
	}
}

func TestNamedEngineRefusalIsFinal(t *testing.T) {
	weakRan := 0
	r := registered(t,
		fakeEngine{name: "weak", kinds: []Kind{Holds}, authority: Observed, result: Result{Claim: ClaimHolds, Strength: Observed}, ran: &weakRan},
		fakeEngine{name: "strong", kinds: []Kind{Holds}, authority: Proved, refusal: errFixtureRefusal},
	)
	plan, err := r.AnswerWith(context.Background(), nil, Question{Kind: Holds}, Budget{}, Only("strong"))
	if err != nil {
		t.Fatalf("answer: %v", err)
	}
	if !sameNames(stepNames(plan), []string{"strong"}) || weakRan != 0 {
		t.Fatalf("steps %v (weak ran %d), want strong alone and no fallback", stepNames(plan), weakRan)
	}
	if !errors.Is(plan.Refused(), errFixtureRefusal) || plan.Result.Covered() || !strings.Contains(plan.Result.Reason, errFixtureRefusal.Error()) {
		t.Fatalf("plan refused %v, result %+v, want the refusal as the answer", plan.Refused(), plan.Result)
	}
	if plan.Selection != Only("strong") {
		t.Fatalf("selection %+v, want the named engine kept on the plan", plan.Selection)
	}
}

func TestNamedEngineNotCoveredIsFinal(t *testing.T) {
	weakRan := 0
	r := registered(t,
		fakeEngine{name: "weak", kinds: []Kind{Holds}, authority: Observed, result: Result{Claim: ClaimHolds, Strength: Observed}, ran: &weakRan},
		fakeEngine{name: "strong", kinds: []Kind{Holds}, authority: Proved, result: Result{Strength: NotCovered, Reason: "the solver answered unknown"}},
	)
	plan, err := r.AnswerWith(context.Background(), nil, Question{Kind: Holds}, Budget{}, Only("strong"))
	if err != nil {
		t.Fatalf("answer: %v", err)
	}
	if !sameNames(stepNames(plan), []string{"strong"}) || weakRan != 0 {
		t.Fatalf("steps %v (weak ran %d), want strong alone and no fallback", stepNames(plan), weakRan)
	}
	if plan.Result.Covered() || plan.Result.Reason != "the solver answered unknown" || plan.Refused() != nil {
		t.Fatalf("result %+v, want strong's not-covered answer to stand", plan.Result)
	}
}

func TestNamedEngineOfAnotherKindRefusesThroughCovers(t *testing.T) {
	f := parseFixture(t)
	q := Question{Kind: Evaluate, Subject: "test::Double", Schedule: policy(t, "reverse"), Perform: func(*runtime.Context) (Answer, error) { return Answer{Claim: ClaimHolds}, nil }}
	plan, err := Default().AnswerWith(context.Background(), Held(f.context(t), nil), q, Budget{}, Only(ExploreEngineName))
	if err != nil {
		t.Fatalf("answer: %v", err)
	}
	if !errors.Is(plan.Refused(), ErrNotAsked) || !sameNames(stepNames(plan), []string{ExploreEngineName}) {
		t.Fatalf("plan %+v, want explore's refusal of an evaluate question", plan)
	}
}

func TestUnknownNamedEngineIsATypedError(t *testing.T) {
	r := registered(t, fakeEngine{name: "a", kinds: []Kind{Holds}, authority: Proved})
	plan, err := r.AnswerWith(context.Background(), nil, Question{Kind: Holds}, Budget{}, Only("z"))
	if !errors.Is(err, ErrUnknownEngine) || len(plan.Steps) != 0 {
		t.Fatalf("answer with z: %v, plan %+v, want the typed error and an empty plan", err, plan)
	}
}

func TestAllRunsEveryCoveringEngineInNameOrder(t *testing.T) {
	var order []string
	engine := func(name string, authority Strength, result Result) fakeEngine {
		return fakeEngine{name: name, kinds: []Kind{Holds}, authority: authority, run: func(context.Context) (Result, error) {
			order = append(order, name)
			return result, nil
		}}
	}
	r := registered(t,
		engine("c", Proved, Result{Claim: ClaimHolds, Strength: Proved}),
		engine("a", Observed, Result{Claim: ClaimHolds, Strength: Observed}),
		fakeEngine{name: "b", kinds: []Kind{Holds}, authority: Bounded, refusal: errFixtureRefusal},
		engine("d", Bounded, Result{Strength: NotCovered, Reason: "the bound was too small"}),
		fakeEngine{name: "e", kinds: []Kind{Sweep}, authority: Observed, result: Result{Claim: ClaimTable, Strength: Observed}},
	)
	plan, err := r.AnswerWith(context.Background(), nil, Question{Kind: Holds}, Budget{}, All())
	if err != nil {
		t.Fatalf("answer: %v", err)
	}
	if !sameNames(order, []string{"a", "c", "d"}) {
		t.Fatalf("ran %v, want every covering engine in name order, not authority order", order)
	}
	if !sameNames(stepNames(plan), []string{"a", "b", "c", "d"}) {
		t.Fatalf("steps %v, want every engine of the kind in name order, the refusal kept", stepNames(plan))
	}
	if plan.Result.Engine != "c" || plan.Result.Strength != Proved || plan.Result.Claim != ClaimHolds {
		t.Fatalf("result %+v, want c's proof to stand as the strongest earned", plan.Result)
	}
	if len(plan.Results()) != 3 || plan.Results()[0].Engine != "a" || plan.Results()[0].Strength != Observed {
		t.Fatalf("results %+v, want a's observed answer kept beside c's", plan.Results())
	}
	if plan.Refused() != nil || len(plan.Disagreements) != 0 {
		t.Fatalf("refused %v, disagreements %+v, want neither", plan.Refused(), plan.Disagreements)
	}
}

func TestAllStopsOnARunError(t *testing.T) {
	fault := errors.New("the process died")
	cRan := 0
	r := registered(t,
		fakeEngine{name: "a", kinds: []Kind{Holds}, authority: Observed, result: Result{Claim: ClaimHolds, Strength: Observed}},
		fakeEngine{name: "b", kinds: []Kind{Holds}, authority: Proved, fault: fault},
		fakeEngine{name: "c", kinds: []Kind{Holds}, authority: Proved, result: Result{Claim: ClaimHolds, Strength: Proved}, ran: &cRan},
	)
	plan, err := r.AnswerWith(context.Background(), nil, Question{Kind: Holds}, Budget{}, All())
	if !errors.Is(err, fault) || cRan != 0 || !sameNames(stepNames(plan), []string{"a", "b"}) {
		t.Fatalf("answer: %v, steps %v, c ran %d; want the fault to stop the plan at b", err, stepNames(plan), cRan)
	}
	if plan.Steps[1].Err != fault || plan.Steps[1].Cancelled {
		t.Fatalf("step %+v, want the fault recorded, not a cancellation", plan.Steps[1])
	}
}

func TestAllKeepsFinishedResultsAndMarksTheCancelled(t *testing.T) {
	cRan := 0
	r := registered(t,
		fakeEngine{name: "a", kinds: []Kind{Holds}, authority: Observed, result: Result{Claim: ClaimHolds, Strength: Observed, Bounds: Bounds{{Name: "steps", Limit: 10}}}},
		fakeEngine{name: "b", kinds: []Kind{Holds}, authority: Proved, run: func(ctx context.Context) (Result, error) {
			<-ctx.Done()
			return Result{}, ctx.Err()
		}},
		fakeEngine{name: "c", kinds: []Kind{Holds}, authority: Proved, result: Result{Claim: ClaimHolds, Strength: Proved}, ran: &cRan},
	)
	plan, err := r.AnswerWith(context.Background(), nil, Question{Kind: Holds}, Budget{Deadline: time.Now().Add(20 * time.Millisecond)}, All())
	if err != nil {
		t.Fatalf("answer: %v, want the finished result to stand", err)
	}
	if !sameNames(stepNames(plan), []string{"a", "b", "c"}) || cRan != 0 {
		t.Fatalf("steps %v (c ran %d), want every engine named and c never run", stepNames(plan), cRan)
	}
	if plan.Steps[0].Cancelled || plan.Steps[0].Result == nil {
		t.Fatalf("step a %+v, want its finished result", plan.Steps[0])
	}
	for _, step := range plan.Steps[1:] {
		if !step.Cancelled || !errors.Is(step.Err, context.DeadlineExceeded) || step.Result != nil {
			t.Fatalf("step %+v, want it cancelled by the deadline", step)
		}
		if len(step.Bounds) != 1 || step.Bounds[0].Name != "deadline" || !step.Bounds[0].Reached || step.Bounds[0].Limit <= 0 || step.Bounds[0].Limit > 20 {
			t.Fatalf("step %s bounds %s, want the deadline reached in milliseconds", step.Engine, step.Bounds)
		}
		if !strings.Contains(step.Standing(), "cancelled at deadline=") || !strings.Contains(step.Standing(), "(reached)") {
			t.Fatalf("standing %q, want the cancelled engine named with the bound it reached", step.Standing())
		}
	}
	if plan.Result.Engine != "a" || plan.Result.Strength != Observed {
		t.Fatalf("result %+v, want the strength the finished run earned", plan.Result)
	}
	if !strings.Contains(plan.Standing(), "all: a holds (observed), b cancelled at deadline=") {
		t.Fatalf("standing %q, want every step listed", plan.Standing())
	}
}

func TestAllWithNothingFinishedFailsWithTheDeadline(t *testing.T) {
	r := registered(t,
		fakeEngine{name: "a", kinds: []Kind{Holds}, authority: Proved, run: func(ctx context.Context) (Result, error) {
			<-ctx.Done()
			return Result{}, ctx.Err()
		}},
		fakeEngine{name: "b", kinds: []Kind{Holds}, authority: Proved, result: Result{Claim: ClaimHolds, Strength: Proved}},
	)
	plan, err := r.AnswerWith(context.Background(), nil, Question{Kind: Holds}, Budget{Deadline: time.Now().Add(10 * time.Millisecond)}, All())
	if !errors.Is(err, context.DeadlineExceeded) || !sameNames(stepNames(plan), []string{"a", "b"}) {
		t.Fatalf("answer: %v, steps %v; want the deadline with both engines cancelled", err, stepNames(plan))
	}
	for _, step := range plan.Steps {
		if !step.Cancelled {
			t.Fatalf("step %+v, want it cancelled", step)
		}
	}
}

func TestAllWithACallerGoneKeepsWhatFinished(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := registered(t,
		fakeEngine{name: "a", kinds: []Kind{Holds}, authority: Observed, result: Result{Claim: ClaimHolds, Strength: Observed}},
		fakeEngine{name: "b", kinds: []Kind{Holds}, authority: Proved, run: func(ctx context.Context) (Result, error) {
			cancel()
			<-ctx.Done()
			return Result{}, ctx.Err()
		}},
	)
	plan, err := r.AnswerWith(ctx, nil, Question{Kind: Holds}, Budget{}, All())
	if err != nil || plan.Result.Engine != "a" {
		t.Fatalf("answer: %v, result %+v; want a's result to stand", err, plan.Result)
	}
	if b := plan.Steps[1]; !b.Cancelled || !errors.Is(b.Err, context.Canceled) || len(b.Bounds) != 0 || b.Standing() != "b cancelled" {
		t.Fatalf("step %+v, want it cancelled with no bound, none having been set", b)
	}
}

func TestAllRefusedByEveryEngineNamesEachRefusal(t *testing.T) {
	other := errors.New("no solver on PATH")
	r := registered(t,
		fakeEngine{name: "a", kinds: []Kind{Holds}, authority: Proved, refusal: errFixtureRefusal},
		fakeEngine{name: "b", kinds: []Kind{Holds}, authority: Proved, refusal: other},
	)
	plan, err := r.AnswerWith(context.Background(), nil, Question{Kind: Holds}, Budget{}, All())
	if err != nil {
		t.Fatalf("answer: %v", err)
	}
	if refused := plan.Refused(); !errors.Is(refused, errFixtureRefusal) || !errors.Is(refused, other) {
		t.Fatalf("refused %v, want both refusals", refused)
	}
	if plan.Result.Covered() || !strings.Contains(plan.Result.Reason, "a refused") || !strings.Contains(plan.Result.Reason, "b refused") {
		t.Fatalf("result %+v, want every refusal named", plan.Result)
	}
}

func TestAllWithNoEngineForTheKindIsATypedError(t *testing.T) {
	r := registered(t, fakeEngine{name: "a", kinds: []Kind{Sweep}, authority: Observed})
	if _, err := r.AnswerWith(context.Background(), nil, Question{Kind: Holds}, Budget{}, All()); !errors.Is(err, ErrNoEngine) {
		t.Fatalf("answer: %v, want no engine", err)
	}
}

func TestAnswerIsAnswerWithUnderAuto(t *testing.T) {
	r := registered(t,
		fakeEngine{name: "weak", kinds: []Kind{Holds}, authority: Observed, result: Result{Claim: ClaimHolds, Strength: Observed}},
		fakeEngine{name: "strong", kinds: []Kind{Holds}, authority: Proved, refusal: errFixtureRefusal},
	)
	plan, err := r.Answer(context.Background(), nil, Question{Kind: Holds}, Budget{})
	if err != nil || plan.Selection != Auto() || !sameNames(stepNames(plan), []string{"strong", "weak"}) {
		t.Fatalf("answer: %v, plan %+v; want auto's plan with the selection recorded", err, plan)
	}
	explicit, err := r.AnswerWith(context.Background(), nil, Question{Kind: Holds}, Budget{}, Auto())
	if err != nil || !sameNames(stepNames(explicit), stepNames(plan)) || explicit.Result.Engine != plan.Result.Engine {
		t.Fatalf("explicit auto %+v, want the same plan as Answer", explicit)
	}
}
