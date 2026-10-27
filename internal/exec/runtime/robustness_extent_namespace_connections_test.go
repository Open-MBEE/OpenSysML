package runtime

import (
	"errors"
	"testing"
)

// TestRuntimeRobustnessExtentNamespaceConnections covers the failure modes of a
// namespace-level connection usage read as an object: an end naming nothing reachable, on one
// connector, on a `[2]` collection or on a connector a `bind` joins to another, connectors bound
// together but attached differently, and an extent reaching any of them must return the typed
// error, leaving no connector behind, never panic.
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

	t.Run("a connector of several objects with an unreachable end is refused whole", func(t *testing.T) {
		src := `package test {
			connection def Link { end a; end b; }
			part sys { part x; part y; }
			connection pair : Link[2] connect sys.x to sys.nothere;
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		_, err := evalIn(t, ctx, pkg.Scope, "pair")
		if !errors.Is(err, errConnectorEnd) {
			t.Fatalf("pair = %v, want errConnectorEnd", err)
		}
		if ids := ctx.occurrences[lookupOne(t, idx, "test::pair")]; len(ids) != 0 {
			t.Errorf("occurrences[pair] = %v after the refusal, want none", ids)
		}
		if n := countInstancesOfType(ctx, "Link"); n != 0 {
			t.Errorf("%d Link left behind, want none", n)
		}
	})

	t.Run("bound connectors attaching to different features are a binding conflict", func(t *testing.T) {
		src := `package test {
			connection def Link { end a; end b; }
			part sys { part x; part y; part z; }
			connection c : Link connect sys.x to sys.y;
			connection d : Link connect sys.x to sys.z;
			bind c = d;
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		_, err := evalIn(t, ctx, pkg.Scope, "c")
		if !errors.Is(err, errBindingConflict) {
			t.Fatalf("c = %v, want errBindingConflict", err)
		}
		if n := countInstancesOfType(ctx, "Link"); n != 0 {
			t.Errorf("%d Link left behind, want none", n)
		}
	})

	t.Run("a bound connector with an unreachable end is refused", func(t *testing.T) {
		src := `package test {
			connection def Link { end a; end b; }
			part sys { part x; part y; }
			connection c : Link connect sys.x to sys.y;
			connection d : Link connect sys.x to sys.nothere;
			bind c = d;
		}`
		ctx, idx := contextForSource(t, src)
		pkg := lookupOne(t, idx, "test")

		for _, expr := range []string{"c", "d", "all Link"} {
			if _, err := evalIn(t, ctx, pkg.Scope, expr); !errors.Is(err, errConnectorEnd) && !errors.Is(err, errBindingConflict) {
				t.Fatalf("%s = %v, want errConnectorEnd or errBindingConflict", expr, err)
			}
		}
		if n := countInstancesOfType(ctx, "Link"); n != 0 {
			t.Errorf("%d Link left behind, want none", n)
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

// countInstancesOfType counts the live objects whose type carries the given name.
func countInstancesOfType(ctx *Context, name string) int {
	n := 0
	for _, inst := range ctx.instances {
		if inst.Type != nil && inst.Type.Name == name {
			n++
		}
	}
	return n
}
