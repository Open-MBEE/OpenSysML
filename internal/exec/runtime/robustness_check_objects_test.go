package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// TestRuntimeRobustnessCheckObjects covers the objects a check constructs where the check fails,
// cannot be evaluated or runs out of room: each ends as a typed error or a verdict, leaving none.
func TestRuntimeRobustnessCheckObjects(t *testing.T) {
	check := func(t *testing.T, ctx *Context, idx *symbols.Index, name string) (CheckResult, error) {
		t.Helper()
		sym := oneSymbol(t, idx, "test::"+name)
		return ctx.CheckRequirementOn(sym, sym.OwnerScope, nil)
	}
	t.Run("repeated_checks_stay_within_the_instance_bound", func(t *testing.T) {
		ctx, idx := libraryShapeContext(t, checkObjectsModel)
		ctx.SetMaxInstances(4)
		for _, name := range []string{"bound", "several", "constructed", "nested", "viaCalc", "subject", "checkedObject"} {
			for i := range 20 {
				if result, err := check(t, ctx, idx, name); err != nil || !result.Holds {
					t.Fatalf("%s, check %d: holds = %v, err = %v; want it to hold within 4 objects", name, i+1, result.Holds, err)
				}
			}
		}
		if got := ctx.InstanceCount(); got != 0 {
			t.Errorf("the checks left %d objects", got)
		}
	})
	t.Run("violated_check_leaves_nothing", func(t *testing.T) {
		ctx, idx := libraryShapeContext(t, checkObjectsModel)
		if _, err := check(t, ctx, idx, "violated"); !errors.Is(err, ErrViolated) {
			t.Errorf("err = %v, want ErrViolated", err)
		}
		if got := ctx.InstanceCount(); got != 0 {
			t.Errorf("the violated check left %d objects", got)
		}
	})
	t.Run("unevaluable_check_leaves_nothing", func(t *testing.T) {
		ctx, idx := libraryShapeContext(t, checkObjectsModel)
		if result, err := check(t, ctx, idx, "unevaluable"); err == nil || errors.Is(err, ErrViolated) {
			t.Errorf("holds = %v, err = %v; want the division by zero reported", result.Holds, err)
		}
		if got := ctx.InstanceCount(); got != 0 {
			t.Errorf("the check that reached no verdict left %d objects", got)
		}
	})
	t.Run("construction_past_the_bound_is_refused", func(t *testing.T) {
		ctx, idx := libraryShapeContext(t, checkObjectsModel)
		ctx.SetMaxInstances(1)
		for i := range 3 {
			if _, err := check(t, ctx, idx, "nested"); !errors.Is(err, ErrInstanceLimitExceeded) {
				t.Fatalf("check %d: err = %v, want ErrInstanceLimitExceeded", i+1, err)
			}
			if got := ctx.InstanceCount(); got != 0 {
				t.Fatalf("check %d: the refused construction left %d objects", i+1, got)
			}
		}
		if result, err := check(t, ctx, idx, "constructed"); err != nil || !result.Holds {
			t.Errorf("holds = %v, err = %v; want a check constructing one object to fit after the refusals", result.Holds, err)
		}
	})
	t.Run("behaving_object_leaves_no_behavior", func(t *testing.T) {
		ctx, idx := libraryShapeContext(t, checkObjectsModel)
		for range 3 {
			if result, err := check(t, ctx, idx, "behavingObject"); err != nil || !result.Holds {
				t.Fatalf("holds = %v, err = %v", result.Holds, err)
			}
		}
		if objects, behaviors := ctx.InstanceCount(), len(ctx.objectBehaviors); objects != 0 || behaviors != 0 {
			t.Errorf("the checks left %d objects running %d behaviors", objects, behaviors)
		}
	})
}
