package analysis

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// barrier releases every engine once n of them have arrived, so a plan that ran them one after
// another would never finish.
type barrier struct {
	mu      sync.Mutex
	n       int
	arrived int
	open    chan struct{}
}

func newBarrier(n int) *barrier { return &barrier{n: n, open: make(chan struct{})} }

func (b *barrier) arrive(ctx context.Context) error {
	b.mu.Lock()
	b.arrived++
	if b.arrived == b.n {
		close(b.open)
	}
	b.mu.Unlock()
	select {
	case <-b.open:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// meeting is an engine of the kind that answers result once every engine on the barrier arrived.
func meeting(name string, b *barrier, result Result) fakeEngine {
	return fakeEngine{name: name, kinds: []Kind{Holds}, authority: Proved, run: func(ctx context.Context) (Result, error) {
		if err := b.arrive(ctx); err != nil {
			return Result{}, err
		}
		return result, nil
	}}
}

func TestAllRunsTheCoveringEnginesConcurrently(t *testing.T) {
	b := newBarrier(3)
	r := registered(t,
		meeting("c", b, Result{Claim: ClaimHolds, Strength: Proved}),
		meeting("a", b, Result{Claim: ClaimHolds, Strength: Observed}),
		meeting("b", b, Result{Claim: ClaimHolds, Strength: Bounded}),
		fakeEngine{name: "d", kinds: []Kind{Holds}, authority: Proved, refusal: errFixtureRefusal},
	)
	budget := Budget{Jobs: 3, Deadline: time.Now().Add(5 * time.Second)}
	plan, err := r.AnswerWith(context.Background(), nil, Question{Kind: Holds}, budget, All())
	if err != nil {
		t.Fatalf("answer: %v, want the three engines to meet and answer", err)
	}
	if !sameNames(stepNames(plan), []string{"a", "b", "c", "d"}) || plan.Steps[3].Refusal == nil {
		t.Fatalf("steps %v, want every engine in name order, d's refusal kept", stepNames(plan))
	}
	if plan.Result.Engine != "c" || plan.Result.Strength != Proved {
		t.Fatalf("result %+v, want c's proof to stand", plan.Result)
	}
}

func TestAllWithFewerJobsThanEnginesRunsThemInTurn(t *testing.T) {
	var mu sync.Mutex
	running, most := 0, 0
	engine := func(name string) fakeEngine {
		return fakeEngine{name: name, kinds: []Kind{Holds}, authority: Observed, run: func(context.Context) (Result, error) {
			mu.Lock()
			running++
			most = max(most, running)
			mu.Unlock()
			time.Sleep(5 * time.Millisecond)
			mu.Lock()
			running--
			mu.Unlock()
			return Result{Claim: ClaimHolds, Strength: Observed}, nil
		}}
	}
	r := registered(t, engine("a"), engine("b"), engine("c"), engine("d"))
	plan, err := r.AnswerWith(context.Background(), nil, Question{Kind: Holds}, Budget{Jobs: 2}, All())
	if err != nil || !sameNames(stepNames(plan), []string{"a", "b", "c", "d"}) {
		t.Fatalf("answer: %v, steps %v; want every engine answered", err, stepNames(plan))
	}
	if most > 2 {
		t.Fatalf("%d engines ran at once, want at most the 2 jobs", most)
	}
}

func TestAllConcurrentFaultStopsThePlanAtTheFaultInNameOrder(t *testing.T) {
	fault := errors.New("the process died")
	aDone := make(chan struct{})
	cRan := 0
	var cCancelled error
	r := registered(t,
		fakeEngine{name: "a", kinds: []Kind{Holds}, authority: Observed, run: func(context.Context) (Result, error) {
			<-aDone
			return Result{Claim: ClaimHolds, Strength: Observed}, nil
		}},
		fakeEngine{name: "b", kinds: []Kind{Holds}, authority: Proved, run: func(context.Context) (Result, error) {
			close(aDone)
			return Result{}, fault
		}},
		fakeEngine{name: "c", kinds: []Kind{Holds}, authority: Proved, ran: &cRan, run: func(ctx context.Context) (Result, error) {
			<-ctx.Done()
			cCancelled = ctx.Err()
			return Result{}, ctx.Err()
		}},
	)
	plan, err := r.AnswerWith(context.Background(), nil, Question{Kind: Holds}, Budget{Jobs: 3}, All())
	if !errors.Is(err, fault) || !sameNames(stepNames(plan), []string{"a", "b"}) {
		t.Fatalf("answer: %v, steps %v; want the fault to stop the plan at b with a's finished step kept", err, stepNames(plan))
	}
	if plan.Steps[0].Result == nil || plan.Steps[1].Err != fault || plan.Steps[1].Cancelled {
		t.Fatalf("steps %+v, want a's result and b's fault", plan.Steps)
	}
	if cRan != 0 && !errors.Is(cCancelled, context.Canceled) {
		t.Fatalf("c saw %v, want its run cancelled by the fault before it", cCancelled)
	}
}

func TestAllConcurrentDeadlineKeepsWhatFinishedAndMarksTheRest(t *testing.T) {
	block := func(name string, authority Strength) fakeEngine {
		return fakeEngine{name: name, kinds: []Kind{Holds}, authority: authority, run: func(ctx context.Context) (Result, error) {
			<-ctx.Done()
			return Result{}, ctx.Err()
		}}
	}
	r := registered(t,
		block("a", Proved),
		fakeEngine{name: "b", kinds: []Kind{Holds}, authority: Observed, result: Result{Claim: ClaimHolds, Strength: Observed}},
		block("c", Proved),
	)
	plan, err := r.AnswerWith(context.Background(), nil, Question{Kind: Holds}, Budget{Jobs: 3, Deadline: time.Now().Add(20 * time.Millisecond)}, All())
	if err != nil || !sameNames(stepNames(plan), []string{"a", "b", "c"}) {
		t.Fatalf("answer: %v, steps %v; want b's finished result to stand", err, stepNames(plan))
	}
	if plan.Result.Engine != "b" || plan.Result.Strength != Observed || plan.Steps[1].Result == nil {
		t.Fatalf("result %+v, want the strength b earned", plan.Result)
	}
	for _, step := range []Step{plan.Steps[0], plan.Steps[2]} {
		if !step.Cancelled || !errors.Is(step.Err, context.DeadlineExceeded) || step.Result != nil {
			t.Fatalf("step %+v, want it cancelled by the deadline", step)
		}
		if len(step.Bounds) != 1 || step.Bounds[0].Name != "deadline" || !step.Bounds[0].Reached {
			t.Fatalf("step %s bounds %s, want the deadline reached", step.Engine, step.Bounds)
		}
	}
}

// An engine serving a universal claim runs to its end however early a witness arrives.
func TestAllDoesNotCancelAUniversalRunWhenAWitnessArrives(t *testing.T) {
	witnessed := make(chan struct{})
	r := registered(t,
		fakeEngine{name: "fast", kinds: []Kind{Holds}, authority: Witnessed, run: func(context.Context) (Result, error) {
			close(witnessed)
			return Result{Claim: ClaimViolated, Strength: Witnessed}, nil
		}},
		fakeEngine{name: "slow", kinds: []Kind{Holds}, authority: Proved, run: func(ctx context.Context) (Result, error) {
			<-witnessed
			if err := ctx.Err(); err != nil {
				return Result{}, err
			}
			return Result{Claim: ClaimHolds, Strength: Observed}, nil
		}},
	)
	plan, err := r.AnswerWith(context.Background(), nil, Question{Kind: Holds}, Budget{Jobs: 2}, All())
	if err != nil {
		t.Fatalf("answer: %v", err)
	}
	for _, step := range plan.Steps {
		if step.Cancelled || step.Result == nil {
			t.Fatalf("step %+v, want it finished", step)
		}
	}
	if plan.Result.Claim != ClaimViolated || plan.Result.Strength != Witnessed {
		t.Fatalf("result %+v, want the witness to stand over the observation", plan.Result)
	}
}

func TestAllPlanCountsTheWorkersOfEveryEngine(t *testing.T) {
	f := parseFixture(t)
	w := &workers{}
	model := f.recording(w)
	q := Question{Kind: Outcomes, Subject: "test::race", Schedule: policy(t, "explore"), Free: FreeSchedule, Linearize: raceRun(t, f)}
	plan, err := Default().AnswerWith(context.Background(), model, q, Budget{Jobs: 2}, All())
	if err != nil {
		t.Fatalf("answer: %v", err)
	}
	if len(plan.Results()) == 0 || plan.Workers == 0 || plan.Workers != w.asked {
		t.Fatalf("plan built %d workers, fixture asked %d times; want every engine's fleet counted", plan.Workers, w.asked)
	}
	if plan.Warming < 0 {
		t.Fatalf("warming %v, want the fleets' warming summed", plan.Warming)
	}
}
