package runtime

import (
	"errors"
	"testing"
)

// orderDependenceTransitiveModel forks to two writers of x in every action and
// then computes from x in some way: a statement, a guard, a cycle of writes, a
// loop that never ends, or a rewrite from a literal.
const orderDependenceTransitiveModel = `package test {
	private import ScalarValues::*;
	action def Derived {
		out x : Integer = 0;
		out y : Integer = 0;
		out z : Integer = 0;
		first start then split;
		fork split;
		first split then a;
		first split then b;
		action a { assign x := 1; }
		action b { assign x := 2; }
		first a then sync;
		first b then sync;
		join sync;
		first sync then derive;
		action derive { assign y := x + 1; assign z := 5; }
		first derive then done;
	}
	action def Guarded {
		out x : Integer = 0;
		out mode : Integer = 0;
		out later : Integer = 0;
		first start then split;
		fork split;
		first split then a;
		first split then b;
		action a { assign x := 1; }
		action b { assign x := 2; }
		first a then sync;
		first b then sync;
		join sync;
		first sync then choose;
		action low { assign mode := 10; }
		action high { assign mode := 20; }
		first low then rejoin;
		first high then rejoin;
		merge rejoin;
		first rejoin then tail;
		action tail { assign later := 7; }
		first tail then done;
		decide choose;
		if x == 1 then low;
		if x == 2 then high;
	}
	action def Cycle {
		out x : Integer = 0;
		out y : Integer = 0;
		out i : Integer = 0;
		first start then split;
		fork split;
		first split then a;
		first split then b;
		action a { assign x := 1; }
		action b { assign x := 2; }
		first a then sync;
		first b then sync;
		join sync;
		first sync then spin;
		action spin {
			while i < 3 {
				assign y := x + 1;
				assign x := y + 1;
				assign i := i + 1;
			}
		}
		first spin then done;
	}
	action def Endless {
		out x : Integer = 0;
		first start then split;
		fork split;
		first split then a;
		first split then b;
		action a { assign x := 1; }
		action b { assign x := 2; }
		first a then sync;
		first b then sync;
		join sync;
		first sync then spin;
		action spin {
			while true {
				assign x := x + 1;
			}
		}
		first spin then done;
	}
	action def Rewritten {
		out x : Integer = 0;
		out y : Integer = 0;
		first start then split;
		fork split;
		first split then a;
		first split then b;
		action a { assign x := 1; }
		action b { assign x := 2; }
		first a then sync;
		first b then sync;
		join sync;
		first sync then reset;
		action reset { assign x := 10; }
		first reset then derive;
		action derive { assign y := x + 1; }
		first derive then done;
	}
}`

// TestRuntimeRobustnessOrderDependenceTransitive: a value computed from an
// order-dependent one stands on the same choice points, through a statement, a
// guard, or a cycle of writes; a value nothing order-dependent reached stays bare;
// a rewrite from a literal clears the mark; a loop that never ends is the typed
// budget error, never a hang, whatever its values stand on.
func TestRuntimeRobustnessOrderDependenceTransitive(t *testing.T) {
	t.Run("a_value_computed_from_an_order_dependent_one_stands_on_its_choice_points", testDerivedValueStandsOnItsChoicePoints)
	t.Run("a_guard_reading_an_order_dependent_value_marks_what_its_branch_writes", testGuardReadMarksItsBranchWrites)
	t.Run("a_cycle_of_dependent_writes_ends_and_marks_both_sides", testCycleOfDependentWritesEndsAndMarksBothSides)
	t.Run("a_budget_limited_loop_is_the_typed_error", testBudgetLimitedLoopIsTheTypedError)
	t.Run("a_rewrite_from_a_literal_clears_the_mark", testRewriteFromALiteralClearsTheMark)
}

// runOrderDependenceTransitive runs the named action of orderDependenceTransitiveModel.
func runOrderDependenceTransitive(t *testing.T, name string, maxSteps int64) (*Context, *ActionExecutor, error) {
	t.Helper()
	m := parseExploreModel(t, orderDependenceTransitiveModel)
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	if maxSteps > 0 {
		ctx.maxSteps = maxSteps
	}
	exec, err := ctx.CreateActionExecutor(m.action(t, name))
	if err != nil {
		t.Fatalf("CreateActionExecutor(%s): %v", name, err)
	}
	err = runWithWatchdog(t, exec.RunToCompletion)
	return ctx, exec, err
}

func testDerivedValueStandsOnItsChoicePoints(t *testing.T) {
	ctx, exec, err := runOrderDependenceTransitive(t, "Derived", 0)
	if err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	x, y := FormatValue(soleElement(exec.Results()["x"])), FormatValue(soleElement(exec.Results()["y"]))
	if (x != "1" || y != "2") && (x != "2" || y != "3") {
		t.Fatalf("x = %s, y = %s, want y = x + 1 for a writer's x", x, y)
	}
	if n := ctx.UnresolvedChoices(); n != 2 {
		t.Errorf("unresolved choice points = %d, want the token order and the write order", n)
	}
	for _, name := range []string{"x", "y", "this.y"} {
		if n := ctx.OrderDependent(name); n != 2 {
			t.Errorf("order-dependent %s = %d choice points, want 2: y was computed from x", name, n)
		}
	}
	if n := ctx.OrderDependent("z"); n != 0 {
		t.Errorf("order-dependent z = %d choice points, want 0: z reads nothing order-dependent", n)
	}
	marked := ctx.OrderDependentAmong([]string{"x", "y", "z"})
	if len(marked) != 2 || marked["x"] != 2 || marked["y"] != 2 {
		t.Errorf("OrderDependentAmong = %v, want x: 2, y: 2", marked)
	}
	for _, c := range ctx.Choices() {
		if c.Independent {
			t.Errorf("%s classified independent; its alternatives all write x", c)
		}
		for _, touched := range c.Touches {
			if touched != "x" {
				t.Errorf("%s touches %v, want x alone: the statement writing y is not a choice", c, c.Touches)
			}
		}
	}
}

func testGuardReadMarksItsBranchWrites(t *testing.T) {
	ctx, exec, err := runOrderDependenceTransitive(t, "Guarded", 0)
	if err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	x, mode := FormatValue(soleElement(exec.Results()["x"])), FormatValue(soleElement(exec.Results()["mode"]))
	if (x != "1" || mode != "10") && (x != "2" || mode != "20") {
		t.Fatalf("x = %s, mode = %s, want the branch the guard on x selected", x, mode)
	}
	if n := ctx.UnresolvedChoices(); n != 2 {
		t.Errorf("unresolved choice points = %d, want 2: one guard held, so the decision drew nothing", n)
	}
	if n := ctx.OrderDependent("mode"); n != 2 {
		t.Errorf("order-dependent mode = %d choice points, want 2: the guard selecting its writer read x", n)
	}
	if n := ctx.OrderDependent("later"); n != 0 {
		t.Errorf("order-dependent later = %d choice points, want 0: written past the merge from a literal", n)
	}
}

func testCycleOfDependentWritesEndsAndMarksBothSides(t *testing.T) {
	ctx, exec, err := runOrderDependenceTransitive(t, "Cycle", 0)
	if err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if got := FormatValue(soleElement(exec.Results()["i"])); got != "3" {
		t.Fatalf("i = %s, want 3 iterations", got)
	}
	for _, name := range []string{"x", "y"} {
		if n := ctx.OrderDependent(name); n != 2 {
			t.Errorf("order-dependent %s = %d choice points, want 2: each is computed from the other", name, n)
		}
	}
	if n := ctx.OrderDependent("i"); n != 0 {
		t.Errorf("order-dependent i = %d choice points, want 0: the counter reads itself alone", n)
	}
}

func testBudgetLimitedLoopIsTheTypedError(t *testing.T) {
	ctx, _, err := runOrderDependenceTransitive(t, "Endless", 300)
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Fatalf("error = %v, want ErrStepLimitExceeded", err)
	}
	if n := ctx.OrderDependent("x"); n != 2 {
		t.Errorf("order-dependent x = %d choice points, want 2: every pass computed x from x", n)
	}
}

func testRewriteFromALiteralClearsTheMark(t *testing.T) {
	ctx, exec, err := runOrderDependenceTransitive(t, "Rewritten", 0)
	if err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if x, y := FormatValue(soleElement(exec.Results()["x"])), FormatValue(soleElement(exec.Results()["y"])); x != "10" || y != "11" {
		t.Fatalf("x = %s, y = %s, want the rewrite and what was computed from it", x, y)
	}
	if n := ctx.UnresolvedChoices(); n != 2 {
		t.Errorf("unresolved choice points = %d, want 2: the racing writes were still drawn", n)
	}
	for _, name := range []string{"x", "y"} {
		if n := ctx.OrderDependent(name); n != 0 {
			t.Errorf("order-dependent %s = %d choice points, want 0: the rewrite read nothing order-dependent", name, n)
		}
	}
	if marked := ctx.OrderDependentAmong([]string{"x", "y"}); len(marked) != 0 {
		t.Errorf("OrderDependentAmong = %v, want nothing marked", marked)
	}
}
