package runtime

import (
	"testing"
)

// TestRuntimeRobustnessExtentNamespacePerformances: a package-level action or state usage
// whose type nests a performance of the same type is read and counted without descending
// forever; one of a count the model does not fix makes no performance up, as a part of that
// count does; one declaring `[0..*]` denotes none and is not an extent's root.
func TestRuntimeRobustnessExtentNamespacePerformances(t *testing.T) {
	t.Run("a recursively typed performance is read and counted lazily", func(t *testing.T) {
		src := `package test {
			action def Act { action again : Act; }
			action a : Act;
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		a, err := evalIn(t, ctx, pkg.Scope, "a")
		if err != nil || a.Kind != ValInstance {
			t.Fatalf("a = %v, %v, want one performance", a, err)
		}
		again, err := evalIn(t, ctx, pkg.Scope, "a.again")
		if err != nil || again.Kind != ValInstance || again.Instance == a.Instance {
			t.Fatalf("a.again = %v, %v, want a performance other than a", again, err)
		}
		all, err := evalIn(t, ctx, pkg.Scope, "all Act")
		if err != nil {
			t.Fatalf("all Act: %v", err)
		}
		if got := len(elementsOf(all)); got != 2 {
			t.Errorf("size(all Act) = %d, want 2 (a and the again it was read through)", got)
		}
	})

	t.Run("a performance of a count the model does not fix is made up for none", func(t *testing.T) {
		src := `package test {
			action def Act;
			part def Prt;
			action some : Act[1..*];
			part parts : Prt[1..*];
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		for _, expr := range []string{"some", "parts"} {
			val, err := evalIn(t, ctx, pkg.Scope, expr)
			if err != nil {
				t.Fatalf("%s: %v", expr, err)
			}
			if val.Kind != ValUndetermined {
				t.Errorf("%s = %s, want <undetermined>: an open count denotes no object of its own", expr, FormatValue(val))
			}
		}
		if n := countInstancesOfType(ctx, "Act"); n != 0 {
			t.Errorf("%d Act made up for an open count, want none", n)
		}
	})

	t.Run("an open-bound performance denotes none and roots no extent", func(t *testing.T) {
		src := `package test {
			private import SequenceFunctions::*;
			action def Act;
			action none : Act[0..*];
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		for _, expr := range []string{"none", "all Act"} {
			val, err := evalIn(t, ctx, pkg.Scope, expr)
			if err != nil {
				t.Fatalf("%s: %v", expr, err)
			}
			if got := len(elementsOf(val)); got != 0 {
				t.Errorf("size(%s) = %d, want 0", expr, got)
			}
		}
		if n := countInstancesOfType(ctx, "Act"); n != 0 {
			t.Errorf("%d Act materialized for an open bound, want none", n)
		}
	})
}
