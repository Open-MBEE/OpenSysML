package runtime

import (
	"errors"
	"testing"
)

// TestRuntimeRobustnessNestedConnectionMultiplicity: a connection usage nested in a part with a
// stated multiplicity whose end is unreachable, names the usage itself, or is reached through an
// extent returns the typed error, leaves no connector behind, and never panics; `[0..*]` holds none.
func TestRuntimeRobustnessNestedConnectionMultiplicity(t *testing.T) {
	t.Run("a collection with an unreachable end is refused whole", func(t *testing.T) {
		src := `package test {
			connection def Link { end a; end b; }
			part sys { part x; part y; connection pair : Link[2] connect x to nothere; }
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		_, err := evalIn(t, ctx, pkg.Scope, "sys.pair")
		if !errors.Is(err, errConnectorEnd) {
			t.Fatalf("sys.pair = %v, want errConnectorEnd", err)
		}
		if n := countInstancesOfType(ctx, "Link"); n != 0 {
			t.Fatalf("%d Link remained after an end was refused, want none", n)
		}
		if _, err := evalIn(t, ctx, pkg.Scope, "sys.pair"); !errors.Is(err, errConnectorEnd) {
			t.Fatalf("second read of sys.pair = %v, want errConnectorEnd again", err)
		}
		if _, err := evalIn(t, ctx, pkg.Scope, "sys.x"); err != nil {
			t.Fatalf("sys.x after the refusal = %v, want the part still readable", err)
		}
	})

	t.Run("an extent reaching the refused collection reports the end", func(t *testing.T) {
		src := `package test {
			connection def Link { end a; end b; }
			part sys { part x; part y; connection pair : Link[2] connect x to nothere; }
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		_, err := evalIn(t, ctx, pkg.Scope, "all Link")
		if !errors.Is(err, errConnectorEnd) {
			t.Fatalf("all Link = %v, want errConnectorEnd", err)
		}
		if n := countInstancesOfType(ctx, "Link"); n != 0 {
			t.Fatalf("%d Link remained after the extent was refused, want none", n)
		}
	})

	t.Run("a collection attaching to itself is refused", func(t *testing.T) {
		src := `package test {
			connection def Link { end a; end b; }
			part sys { part x; connection pair : Link[2] connect pair to x; }
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		_, err := evalIn(t, ctx, pkg.Scope, "sys.pair")
		if !errors.Is(err, ErrCyclicFeatureValue) {
			t.Fatalf("sys.pair = %v, want ErrCyclicFeatureValue", err)
		}
		if n := countInstancesOfType(ctx, "Link"); n != 0 {
			t.Fatalf("%d Link remained after the cycle was refused, want none", n)
		}
	})

	t.Run("an unbounded optional collection holds no connector", func(t *testing.T) {
		src := `package test {
			connection def Link { end a; end b; }
			part sys { part x; part y; connection none : Link[0..*] connect x to y; }
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		got, err := evalIn(t, ctx, pkg.Scope, "sys.none")
		if err != nil || got.Kind != ValSequence || len(got.Sequence().Elements()) != 0 {
			t.Fatalf("sys.none = %v, %v; want an empty sequence", got, err)
		}
		if n := countInstancesOfType(ctx, "Link"); n != 0 {
			t.Fatalf("%d Link materialized for [0..*], want none", n)
		}
	})
}
