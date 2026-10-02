package runtime

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// TestRuntimeRobustnessExtentModelDetermined covers the failure and determinism modes of
// the namespace-owned usages an extent reaches: a binding connector between objects must
// refuse ends whose values differ or that resolve through each other, an abstract
// collection must refuse the subsetters that leave it short of its lower bound, the
// objects `all T` enumerates must not depend on the order usages were read, a binding
// must carry into an adopted context, and a probe must unwind one on rollback.
func TestRuntimeRobustnessExtentModelDetermined(t *testing.T) {
	t.Run("a namespace binding refusing conflicting ends leaves nothing behind", func(t *testing.T) {
		src := `package test {
			part def Car;
			part a : Car = new Car();
			part b : Car = new Car();
			bind a = b;
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		_, err := evalIn(t, ctx, pkg.Scope, "a")
		if !errors.Is(err, ErrBindingConflict) {
			t.Fatalf("a = %v, want binding conflict", err)
		}
		for _, name := range []string{"test::a", "test::b"} {
			sym := lookupOne(t, idx, name)
			if ids := ctx.occurrences[sym]; len(ids) != 0 {
				t.Fatalf("occurrences[%s] = %v, want none: the refused binding records no occurrences", name, ids)
			}
		}
		if len(ctx.instances) != 2 {
			t.Fatalf("%d objects materialized, want the two stated values only", len(ctx.instances))
		}
	})

	t.Run("a namespace binding cycle is refused", func(t *testing.T) {
		src := `package test {
			part def Car;
			part a : Car = b;
			part b : Car = a;
			bind a = b;
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		_, err := evalIn(t, ctx, pkg.Scope, "a")
		if !errors.Is(err, ErrCyclicFeatureValue) {
			t.Fatalf("a = %v, want cyclic feature value dependency", err)
		}
	})

	t.Run("an abstract collection under-count is refused", func(t *testing.T) {
		src := `package test {
			part def Car;
			abstract part vs : Car[2];
			part c1 : Car[1] :> vs;
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		_, err := evalIn(t, ctx, pkg.Scope, "vs")
		if !errors.Is(err, ErrMultiplicityViolation) {
			t.Fatalf("vs = %v, want multiplicity violation", err)
		}
		if !strings.Contains(err.Error(), "vs") {
			t.Fatalf("vs = %v, want the violation naming vs", err)
		}
	})

	t.Run("the extent is the same cold and after the members were read", func(t *testing.T) {
		src := `package test {
			part def Car;
			abstract part vs : Car[1..*];
			part c1 : Car[1] :> vs;
			part c2 : Car[1] :> vs;
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		cold, err := evalIn(t, ctx, pkg.Scope, "all test::Car")
		if err != nil {
			t.Fatalf("all test::Car: %v", err)
		}
		coldIDs := heldObjects(cold)
		for _, expr := range []string{"c1", "c2", "vs"} {
			if _, err := evalIn(t, ctx, pkg.Scope, expr); err != nil {
				t.Fatalf("%s: %v", expr, err)
			}
		}
		warm, err := evalIn(t, ctx, pkg.Scope, "all test::Car")
		if err != nil {
			t.Fatalf("all test::Car: %v", err)
		}
		warmIDs := heldObjects(warm)
		if !slices.Equal(warmIDs, coldIDs) {
			t.Fatalf("extent after the reads = %v, cold extent = %v", warmIDs, coldIDs)
		}
	})

	t.Run("adoption keeps a namespace binding", func(t *testing.T) {
		src := `package test {
			part def Car;
			part a : Car;
			part b : Car;
			bind a = b;
		}`
		prev, _ := contextForSource(t, src)
		prevPkg := lookupOne(t, prev.model.resolver.Index(), "test")
		got, err := evalIn(t, prev, prevPkg.Scope, "a")
		if err != nil {
			t.Fatalf("a: %v", err)
		}
		obj := prev.instances[heldObjects(got)[0]]

		ctx := contextOver(t, src)
		pkg := lookupOne(t, ctx.model.resolver.Index(), "test")
		if _, err := ctx.Adopt(prev, prev.ShapesOf(obj), obj); err != nil {
			t.Fatalf("Adopt: %v", err)
		}
		carried, err := evalIn(t, ctx, pkg.Scope, "a")
		if err != nil {
			t.Fatalf("a after adoption: %v", err)
		}
		if heldObjects(carried)[0] != obj.ID {
			t.Fatalf("a after adoption = object %d, want carried object %d", heldObjects(carried)[0], obj.ID)
		}
		same, err := evalIn(t, ctx, pkg.Scope, "a === b")
		if err != nil {
			t.Fatalf("a === b: %v", err)
		}
		if FormatValue(same) != "true" {
			t.Fatalf("a === b after adoption = %s, want true", FormatValue(same))
		}
	})

	t.Run("a probe undo forgets a namespace binding", func(t *testing.T) {
		src := `package test {
			part def Car;
			part a : Car;
			part b : Car;
			bind a = b;
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		end := ctx.beginProbe()
		if _, err := evalIn(t, ctx, pkg.Scope, "a"); err != nil {
			t.Fatalf("a: %v", err)
		}
		end()
		if len(ctx.namespaceBindings) != 0 {
			t.Fatalf("namespace bindings after undo = %v, want none", ctx.namespaceBindings)
		}
		same, err := evalIn(t, ctx, pkg.Scope, "a === b")
		if err != nil {
			t.Fatalf("a === b: %v", err)
		}
		if FormatValue(same) != "true" {
			t.Fatalf("a === b after undo = %s, want true", FormatValue(same))
		}
	})
}
