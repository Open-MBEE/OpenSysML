package runtime

import (
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// An interrupt flag raised while a run is under way stops it at its next step
// with ErrInterrupted, whatever budget the run had left, and the flag is the
// caller's: cleared again, the next run is unaffected.
func TestRuntimeRobustnessInterrupt(t *testing.T) {
	t.Run("ActionRunStopsAtItsNextStep", testInterruptStopsAnActionRun)
	t.Run("ActionRunStopsFromAnotherGoroutine", testInterruptFromAnotherGoroutine)
	t.Run("EvaluationStopsAtItsNextStep", testInterruptStopsAnEvaluation)
	t.Run("StateDoBehaviorStopsAtItsNextStep", testInterruptStopsAStateDoBehavior)
	t.Run("InterruptionIsNotTheBehaviorsFailure", testInterruptIsNotRecordedOnABehavior)
	t.Run("InterruptedDeferredStartIsRecordedFailed", testInterruptFailsADeferredStart)
}

const spinningAction = `
	package test {
		action spin {
			first start;
			merge m;
			action a;
			succession first start then m;
			succession first m then a;
			succession first a then m;
		}
	}
`

func testInterruptStopsAnActionRun(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, spinningAction))
	ctx.maxActionSteps = 1_000_000
	flag := new(atomic.Bool)
	ctx.SetInterrupt(flag)
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "spin", ast.DefAction)
	if sym == nil {
		t.Fatal("action spin not found")
	}
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	flag.Store(true)
	if err := exec.RunToCompletion(); !errors.Is(err, ErrInterrupted) {
		t.Fatalf("RunToCompletion() = %v, want ErrInterrupted", err)
	}
	if exec.State() == StateCompleted {
		t.Error("an interrupted run reports itself completed")
	}

	// With the flag cleared, a fresh run is bounded by its budget alone.
	flag.Store(false)
	ctx.maxActionSteps = 100
	again, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	if err := again.RunToCompletion(); !errors.Is(err, ErrActionStepLimitExceeded) {
		t.Fatalf("RunToCompletion() after the flag cleared = %v, want ErrActionStepLimitExceeded", err)
	}
}

func testInterruptFromAnotherGoroutine(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, spinningAction))
	ctx.maxActionSteps = 1 << 40
	flag := new(atomic.Bool)
	ctx.SetInterrupt(flag)
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "spin", ast.DefAction)
	if sym == nil {
		t.Fatal("action spin not found")
	}
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- exec.RunToCompletion() }()
	time.Sleep(50 * time.Millisecond)
	flag.Store(true)
	select {
	case err := <-done:
		if !errors.Is(err, ErrInterrupted) {
			t.Fatalf("RunToCompletion() = %v, want ErrInterrupted", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the run did not stop within ten seconds of the interrupt")
	}
}

func testInterruptStopsAnEvaluation(t *testing.T) {
	_, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test { attribute x = 1; }`))
	flag := new(atomic.Bool)
	ctx.SetInterrupt(flag)
	flag.Store(true)
	if _, err := ctx.Eval(parseExpr(t, "1 + 2")); !errors.Is(err, ErrInterrupted) {
		t.Fatalf("Eval() = %v, want ErrInterrupted", err)
	}
	flag.Store(false)
	if _, err := ctx.Eval(parseExpr(t, "1 + 2")); err != nil {
		t.Fatalf("Eval() after the flag cleared = %v", err)
	}
}

func testInterruptStopsAStateDoBehavior(t *testing.T) {
	src := `
		package test {
			state def Machine {
				entry; then busy;
				state busy {
					do action spin {
						first start;
						merge m;
						action a;
						succession first start then m;
						succession first m then a;
						succession first a then m;
					}
				}
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	ctx.maxDoSteps = 1_000_000
	flag := new(atomic.Bool)
	ctx.SetInterrupt(flag)
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("state def Machine not found")
	}
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		t.Fatalf("create state executor: %v", err)
	}
	flag.Store(true)
	if err := exec.RunToQuiescence(); !errors.Is(err, ErrInterrupted) {
		t.Fatalf("RunToQuiescence() = %v, want ErrInterrupted", err)
	}
}

// testInterruptIsNotRecordedOnABehavior checks a run of a type-bound behavior
// stopped from outside is not recorded as that behavior's failure, which would
// end it for every later run, while a failure of its own still is.
func testInterruptIsNotRecordedOnABehavior(t *testing.T) {
	bound := &ObjectBehavior{typeBound: true}
	if recordsFailure(bound, fmt.Errorf("machine: %w", ErrInterrupted)) {
		t.Error("an interrupted run was recorded as the behavior's failure")
	}
	if !recordsFailure(bound, errors.New("the model failed")) {
		t.Error("a behavior's own failure was not recorded on it")
	}
}

// testInterruptFailsADeferredStart checks a held behavior whose start is stopped
// from outside is recorded failed rather than left neither started nor failed,
// which later runs would pass over in silence.
func testInterruptFailsADeferredStart(t *testing.T) {
	src := `package test {
		attribute def Wake;
		state def Earlier {
			entry; then waiting;
			state waiting;
			transition first waiting accept Wake then done;
		}
		state def Later {
			entry; then done;
		}
		part def Holder {
			exhibit state earlier : Earlier;
			exhibit state later : Later;
			first earlier then later;
		}
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "test::Holder"))
	if err != nil {
		t.Fatalf("instantiate Holder: %v", err)
	}
	later := behaviorNamed(t, inst, "later")
	if later.deferred == nil {
		t.Fatal("later exhibit was not held while earlier awaits Wake")
	}
	var stop atomic.Bool
	ctx.SetInterrupt(&stop)
	stop.Store(true)
	err = ctx.releaseDeferredBehavior(later)
	stop.Store(false)
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("release of the held exhibit under a raised flag = %v, want ErrInterrupted", err)
	}
	if later.deferred != nil {
		t.Error("the interrupted start is still held")
	}
	if !errors.Is(later.Err, ErrStartInterrupted) {
		t.Errorf("the interrupted start recorded %v on the behavior, want ErrStartInterrupted", later.Err)
	}
	if errors.Is(later.Err, ErrInterrupted) {
		t.Error("the recorded failure reads as an interruption of the run that next asks for the behavior")
	}
}
