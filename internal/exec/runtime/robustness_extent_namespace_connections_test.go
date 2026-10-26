package runtime

import (
	"errors"
	"testing"
)

// TestRuntimeRobustnessExtentNamespaceConnections covers the failure modes of a
// namespace-level connection usage read as an object: an end naming nothing reachable, a
// connector of several objects and an extent that reaches either must return the typed
// connector error, leaving no connector behind, never panic.
func TestRuntimeRobustnessExtentNamespaceConnections(t *testing.T) {
	t.Run("an end naming an unresolvable feature is refused", func(t *testing.T) {
		src := `package test {
			connection def Link { end a; end b; }
			part sys { part x; part y; }
			connection outer : Link connect sys.x to sys.missing;
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		_, err := evalIn(t, ctx, pkg.Scope, "outer")
		if !errors.Is(err, errConnectorEnd) {
			t.Fatalf("outer = %v, want errConnectorEnd", err)
		}
		if ids := ctx.occurrences[lookupOne(t, idx, "test::outer")]; len(ids) != 0 {
			t.Fatalf("occurrences[outer] = %v, want none: the refused connector records no occurrence", ids)
		}
		for _, inst := range ctx.instances {
			if inst.Type != nil && inst.Type.Name == "Link" {
				t.Fatalf("a Link remained after its end was refused")
			}
		}
	})

	t.Run("an extent reaching the refused connector reports the end", func(t *testing.T) {
		src := `package test {
			connection def Link { end a; end b; }
			part sys { part x; part y; connection inner : Link connect x to y; }
			connection outer : Link connect sys.x to sys.missing;
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		_, err := evalIn(t, ctx, pkg.Scope, "all Link")
		if !errors.Is(err, errConnectorEnd) {
			t.Fatalf("all Link = %v, want errConnectorEnd", err)
		}
	})

	t.Run("a connector of several objects is refused as a nested one is", func(t *testing.T) {
		src := `package test {
			connection def Link { end a; end b; }
			part sys { part x; part y; }
			connection pair : Link[2] connect sys.x to sys.y;
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		_, err := evalIn(t, ctx, pkg.Scope, "pair")
		if !errors.Is(err, errConnectorEnd) {
			t.Fatalf("pair = %v, want errConnectorEnd", err)
		}
		if _, err := evalIn(t, ctx, pkg.Scope, "all Link"); !errors.Is(err, errConnectorEnd) {
			t.Fatalf("all Link = %v, want errConnectorEnd", err)
		}
	})

	t.Run("an end naming the connector itself is refused", func(t *testing.T) {
		src := `package test {
			connection def Link { end a; end b; }
			part sys { part x; }
			connection loop : Link connect loop to sys.x;
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		_, err := evalIn(t, ctx, pkg.Scope, "loop")
		if !errors.Is(err, errConnectorEnd) && !errors.Is(err, ErrCyclicFeatureValue) {
			t.Fatalf("loop = %v, want errConnectorEnd or ErrCyclicFeatureValue", err)
		}
	})
}
