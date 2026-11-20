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
	// indirectlyHeld holds a constructed object only through a stored function and a referenced variant.
	const indirectlyHeld = `
		package test {
			private import ScalarValues::*;
			part def P { attribute x : Integer = 3; calc scale { in n : Integer[1]; return : Integer = x * n; } }
			part def Engine { attribute power : Real default = 10.0; }
			abstract part def Powered {
				variation part engine : Engine { variant part petrol : Engine; variant part electric : Engine; }
			}
			part def Tank :> Powered { part :>> engine = engine::electric; }
			part def W {
				ref e : Engine = (new Tank()).engine;
				attribute f = (new P()).scale;
				assert constraint viaVariant { e.power > 0.0 }
				assert constraint viaFunction { f != null }
			}
			part w : W;
		}
	`
	t.Run("stored_function_keeps_its_receiver", func(t *testing.T) {
		ctx, idx := libraryShapeContext(t, indirectlyHeld)
		ctx.SetMaxInstances(2)
		w := mustInstantiate(t, ctx, oneSymbol(t, idx, "test::w"))
		typ := oneSymbol(t, idx, "test::W")
		sym := memberNamed(t, typ, "viaFunction")
		for i := range 5 {
			if result, err := ctx.CheckConstraintOn(sym, typ.Scope, w); err != nil || !result.Holds {
				t.Fatalf("check %d: holds = %v, err = %v", i+1, result.Holds, err)
			}
		}
		fn := mustEvalOn(t, ctx, w, "f")
		if fn.Kind != ValFunction || fn.FunctionSelf() == nil {
			t.Fatalf("w.f = %s, want a function of the constructed object", FormatValue(fn))
		}
		if held, live := ctx.Instance(fn.FunctionSelf().ID); !live || held != fn.FunctionSelf() {
			t.Errorf("w.f's receiver #%d was released with the check that derived it", fn.FunctionSelf().ID)
		}
		if got := ctx.InstanceCount(); got != 2 {
			t.Errorf("%d objects stand, want w and the receiver", got)
		}
	})
	t.Run("referenced_variant_keeps_its_selector", func(t *testing.T) {
		ctx, idx := libraryShapeContext(t, indirectlyHeld)
		ctx.SetMaxInstances(3)
		w := mustInstantiate(t, ctx, oneSymbol(t, idx, "test::w"))
		typ := oneSymbol(t, idx, "test::W")
		sym := memberNamed(t, typ, "viaVariant")
		var first Value
		for i := range 5 {
			if result, err := ctx.CheckConstraintOn(sym, typ.Scope, w); err != nil || !result.Holds {
				t.Fatalf("check %d: holds = %v, err = %v", i+1, result.Holds, err)
			}
			engine := mustEvalOn(t, ctx, w, "e")
			if _, live := ctx.Instance(engine.Instance); engine.Kind != ValVariant || !live {
				t.Fatalf("check %d: w.e = %s, want the variant standing", i+1, FormatValue(engine))
			}
			if i == 0 {
				first = engine
			} else if engine.Instance != first.Instance {
				t.Fatalf("check %d: w.e = %s, want %s still", i+1, FormatValue(engine), FormatValue(first))
			}
		}
		if got := ctx.InstanceCount(); got != 3 {
			t.Errorf("%d objects stand, want w, the tank and the engine it selected", got)
		}
	})
}
