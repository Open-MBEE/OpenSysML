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
