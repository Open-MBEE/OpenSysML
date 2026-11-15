package runtime

import (
	"errors"
	"testing"
)

// A loop an action performs and each iteration of its body are performance objects
// of the run (loop_performances.go); the bounds the run keeps hold over them.
const loopPerformancesModel = `package test {
	private import ScalarValues::Integer;
	part def K {
		attribute n : Integer = 0;
		perform action loopy {
			first start;
			then loop action body {
				first start;
				then action inc assign n := n + 1;
				then done;
			} until n >= 5;
			then done;
		}
	}
	part k : K;
}`

func TestRuntimeRobustnessLoopPerformances(t *testing.T) {
	t.Run("an_object_bound_an_iteration_reaches_is_the_typed_error", testLoopIterationPastObjectBound)
	t.Run("a_step_budget_the_iterations_spend_is_the_typed_error", testLoopIterationPastStepBudget)
	t.Run("the_loop_and_its_bodies_have_ended_with_the_run", testLoopPerformancesEndWithTheRun)
	t.Run("an_object_bound_a_definition_run_reaches_is_the_typed_error", testDefinitionLoopPastObjectBound)
	t.Run("a_step_budget_a_definition_run_spends_is_the_typed_error", testDefinitionLoopPastStepBudget)
	t.Run("an_object_bound_a_case_loop_reaches_is_the_typed_error", testCaseLoopPastObjectBound)
	t.Run("an_assertion_the_held_loops_fail_is_the_typed_error", testAssertionOverHeldLoops)
}

// An assertion a definition run reaches reads the loops the run holds by then.
const assertedLoopPerformancesModel = `package test {
	private import ScalarValues::Integer;
	private import SequenceFunctions::size;
	action def Held {
		attribute n : Integer = 0;
		first start;
		then while n < 2 { assign n := n + 1; }
		then assert constraint twice { size(loops.body) == 2 }
		then done;
	}
	action def Missed {
		attribute n : Integer = 0;
		first start;
		then while n < 2 { assign n := n + 1; }
		then assert constraint thrice { size(loops.body) == 3 }
		then done;
	}
}`

func testAssertionOverHeldLoops(t *testing.T) {
	ctx, idx := libraryModelContext(t, assertedLoopPerformancesModel)
	run := func(name string) error {
		exec, err := ctx.CreateActionExecutor(lookupOne(t, idx, name))
		if err != nil {
			t.Fatalf("create action executor for %s: %v", name, err)
		}
		return exec.RunToCompletion()
	}
	if err := run("test::Held"); err != nil {
		t.Fatalf("an assertion the held iterations satisfy: %v", err)
	}
	var violation *ViolationError
	if err := run("test::Missed"); !errors.As(err, &violation) {
		t.Fatalf("an assertion the held iterations fail: got %v, want a ViolationError", err)
	}
}

// A definition performed on its own holds the loops its body reaches, nested ones
// under the iteration that reached them; a case holds those of its body.
const definitionLoopPerformancesModel = `package test {
	private import ScalarValues::Integer;
	action def Run {
		attribute n : Integer = 0;
		attribute t : Integer = 0;
		first start;
		then while n < 5 {
			assign n := n + 1;
			for i in (1, 2) {
				action add assign t := t + i;
			}
		}
		then done;
	}
	analysis def Counted {
		subject s : Integer = 0;
		attribute n : Integer = 0;
		loop action body {
			action inc assign n := n + 1;
		} until n >= 5;
		return r : Integer = n;
	}
	part def K {
		analysis counted : Counted;
		attribute r : Integer = counted.r;
	}
	part k : K;
}`

func definitionLoopRun(t *testing.T, budget func(*Context)) (*Context, error) {
	t.Helper()
	ctx, idx := libraryModelContext(t, definitionLoopPerformancesModel)
	exec, err := ctx.CreateActionExecutor(lookupOne(t, idx, "test::Run"))
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	budget(ctx)
	return ctx, exec.RunToCompletion()
}

func testDefinitionLoopPastObjectBound(t *testing.T) {
	ctx, err := definitionLoopRun(t, func(ctx *Context) { ctx.SetMaxInstances(6) })
	if !errors.Is(err, ErrInstanceLimitExceeded) {
		t.Fatalf("Run under a bound of 6 objects: err = %v, want ErrInstanceLimitExceeded", err)
	}
	if got := ctx.InstanceCount(); got > 6 {
		t.Errorf("InstanceCount = %d after the refused iteration, want at most 6", got)
	}
}

func testDefinitionLoopPastStepBudget(t *testing.T) {
	_, err := definitionLoopRun(t, func(ctx *Context) { ctx.maxSteps = 12 })
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Fatalf("Run under a budget of 12 steps: err = %v, want ErrStepLimitExceeded", err)
	}
}

func testCaseLoopPastObjectBound(t *testing.T) {
	ctx, idx := libraryModelContext(t, definitionLoopPerformancesModel)
	ctx.SetMaxInstances(5)
	k, err := ctx.Instantiate(lookupOne(t, idx, "test::k"))
	if err == nil {
		_, err = k.GetFeatureValue(ctx, "r")
	}
	if !errors.Is(err, ErrInstanceLimitExceeded) {
		t.Fatalf("k.r under a bound of 5 objects: err = %v, want ErrInstanceLimitExceeded", err)
	}
	if got := ctx.InstanceCount(); got > 5 {
		t.Errorf("InstanceCount = %d after the refused iteration, want at most 5", got)
	}
}

func testLoopIterationPastObjectBound(t *testing.T) {
	ctx, idx := libraryModelContext(t, loopPerformancesModel)
	ctx.SetMaxInstances(4)
	_, err := ctx.Instantiate(lookupOne(t, idx, "test::k"))
	if !errors.Is(err, ErrInstanceLimitExceeded) {
		t.Fatalf("Instantiate(k) under a bound of 4 objects: err = %v, want ErrInstanceLimitExceeded", err)
	}
	if got := ctx.InstanceCount(); got > 4 {
		t.Errorf("InstanceCount = %d after the refused iteration, want at most 4", got)
	}
}

func testLoopIterationPastStepBudget(t *testing.T) {
	ctx, idx := libraryModelContext(t, loopPerformancesModel)
	ctx.maxSteps = 12
	_, err := ctx.Instantiate(lookupOne(t, idx, "test::k"))
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Fatalf("Instantiate(k) under a budget of 12 steps: err = %v, want ErrStepLimitExceeded", err)
	}
}

func testLoopPerformancesEndWithTheRun(t *testing.T) {
	ctx, idx := libraryModelContext(t, loopPerformancesModel)
	k, err := ctx.Instantiate(lookupOne(t, idx, "test::k"))
	if err != nil {
		t.Fatalf("Instantiate(k): %v", err)
	}
	loopy, err := k.GetFeatureValue(ctx, "loopy")
	if err != nil {
		t.Fatalf("loopy: %v", err)
	}
	loopyID, _ := loopy.HeldValue().Object()
	loopyInst, _ := ctx.Instance(loopyID)
	loops, err := loopyInst.GetFeatureValue(ctx, "loops")
	if err != nil {
		t.Fatalf("loopy.loops: %v", err)
	}
	held := elementsOf(loops.HeldValue())
	if len(held) != 1 {
		t.Fatalf("loopy.loops holds %d objects, want the one loop", len(held))
	}
	loopID, _ := held[0].Object()
	loop, _ := ctx.Instance(loopID)
	body, err := loop.GetFeatureValue(ctx, "body")
	if err != nil {
		t.Fatalf("loop.body: %v", err)
	}
	bodies := elementsOf(body.HeldValue())
	if len(bodies) != 5 {
		t.Fatalf("loop.body holds %d objects, want one per iteration (5)", len(bodies))
	}
	for _, id := range append([]Value{held[0]}, bodies...) {
		object, _ := id.Object()
		life, ok := ctx.OccurrenceLife(object)
		if !ok || life.Began == 0 || life.Ended == 0 {
			t.Errorf("object #%d life = %+v (known %v), want begun and ended with the run", object, life, ok)
		}
	}
	for i := 1; i < len(bodies); i++ {
		before, _ := ctx.OccurrenceLife(mustObject(bodies[i-1]))
		after, _ := ctx.OccurrenceLife(mustObject(bodies[i]))
		if before.Ended > after.Began {
			t.Errorf("body #%d ended at %d after body #%d began at %d: iterations are in sequence", i, before.Ended, i+1, after.Began)
		}
	}
}

func mustObject(v Value) int64 {
	id, _ := v.Object()
	return id
}
