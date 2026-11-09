package runtime

import (
	"fmt"
	"strings"
	"testing"
)

// reachModel holds one port object under two paths: the tank's `fuelOut` and
// the interface end `supplier` that references it, and the engine's `fuelIn`
// under `consumer`. The vehicle's `pump` is a reference to the tank.
const reachModel = `package Fuel {
	port def FuelPort { out item fuel; }
	interface def FuelLine { end supplier : FuelPort; end consumer : ~FuelPort; }
	part def Tank { port fuelOut : FuelPort; }
	part def Engine { port fuelIn : ~FuelPort; }
	part def Powertrain { part tank : Tank; part engine : Engine; }
	part def Wheel;
	part vehicle {
		part powertrain : Powertrain;
		part wheels : Wheel[2];
		ref part pump : Tank = powertrain.tank;
		interface fuelLine : FuelLine connect supplier ::> powertrain.tank.fuelOut to consumer ::> powertrain.engine.fuelIn;
	}
}`

func reachPaths(reached Reached) []string {
	paths := make([]string, 0, len(reached.Reaches))
	for _, r := range reached.Reaches {
		paths = append(paths, r.Path())
	}
	return paths
}

func pathTexts(paths Paths) []string {
	texts := make([]string, 0, len(paths.Steps))
	for _, steps := range paths.Steps {
		texts = append(texts, PathText(steps))
	}
	return texts
}

// Traverse reaches every object once, the root first, depth-first over the
// features in name order: the shared port is reached under the interface end
// (fuelLine sorts before powertrain) and the holder it reports is that end;
// the tank's `fuelOut` and the reference `pump`, being those objects, are not
// reached again.
func TestTraverseReachesEachObjectOnceInNameOrder(t *testing.T) {
	ctx, idx := contextForSource(t, reachModel)
	root, err := ctx.Instantiate(lookupOne(t, idx, "Fuel::vehicle"))
	if err != nil {
		t.Fatal(err)
	}
	reached := ctx.Traverse(root, TraversalBounds{Depth: 8, Objects: 1000})
	if reached.Truncated {
		t.Fatal("truncated")
	}
	want := []string{
		"",
		"fuelLine",
		"fuelLine.consumer",
		"fuelLine.consumer.fuel",
		"fuelLine.supplier",
		"fuelLine.supplier.fuel",
		"powertrain",
		"powertrain.engine",
		"powertrain.tank",
		"wheels[1]",
		"wheels[2]",
	}
	got := reachPaths(reached)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("reached\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	seen := map[int64]bool{}
	for i, r := range reached.Reaches {
		if seen[r.Object.ID] {
			t.Errorf("object #%d reached twice", r.Object.ID)
		}
		seen[r.Object.ID] = true
		if i == 0 {
			if r.Holder != nil || r.Feature != nil || r.Depth() != 0 {
				t.Errorf("root reached through %v.%v", r.Holder, r.Feature)
			}
			continue
		}
		if r.Holder == nil || r.Feature == nil || r.Steps[len(r.Steps)-1].Object != r.Object {
			t.Errorf("%s: holder %v, feature %v", r.Path(), r.Holder, r.Feature)
		}
	}
	supplier := reached.Reaches[4]
	if supplier.Holder.ID != reached.Reaches[1].Object.ID || supplier.Feature.Name != "supplier" || supplier.Index != 0 {
		t.Errorf("supplier reached through #%d.%s[%d]", supplier.Holder.ID, supplier.Feature.Name, supplier.Index)
	}
	second := reached.Reaches[10]
	if second.Feature.Name != "wheels" || second.Index != 2 || second.Holder.ID != root.ID {
		t.Errorf("second wheel reached through #%d.%s[%d]", second.Holder.ID, second.Feature.Name, second.Index)
	}
}

// Every path to the shared port is listed: the tank's own port, the interface
// end referencing it, and the same two below the reference to the tank. The
// root is reached by the empty path alone.
func TestPathsToListsEveryAlias(t *testing.T) {
	ctx, idx := contextForSource(t, reachModel)
	root, err := ctx.Instantiate(lookupOne(t, idx, "Fuel::vehicle"))
	if err != nil {
		t.Fatal(err)
	}
	reached := ctx.Traverse(root, TraversalBounds{})
	byPath := map[string]*Instance{}
	for _, r := range reached.Reaches {
		byPath[r.Path()] = r.Object
	}
	bounds := PathBounds{Depth: 8, Paths: 100}
	cases := map[string][]string{
		"":                  {""},
		"fuelLine.supplier": {"fuelLine.supplier", "powertrain.tank.fuelOut", "pump.fuelOut"},
		"fuelLine.consumer": {"fuelLine.consumer", "powertrain.engine.fuelIn"},
		"powertrain.tank":   {"powertrain.tank", "pump"},
		"wheels[2]":         {"wheels[2]"},
	}
	for path, want := range cases {
		target, ok := byPath[path]
		if !ok {
			t.Fatalf("no object at %q", path)
		}
		got := ctx.PathsTo(root, target, bounds)
		if got.Truncated {
			t.Errorf("%s: truncated", path)
		}
		if texts := pathTexts(got); strings.Join(texts, " ") != strings.Join(want, " ") {
			t.Errorf("paths to %q = %v, want %v", path, texts, want)
		}
	}
	for _, steps := range ctx.PathsTo(root, byPath["fuelLine.supplier"], bounds).Steps {
		if len(steps) > 0 && steps[len(steps)-1].Object != byPath["fuelLine.supplier"] {
			t.Errorf("path %s ends elsewhere", PathText(steps))
		}
	}
}

// The bounds cut the search short and say so.
func TestPathsToHonoursBounds(t *testing.T) {
	ctx, idx := contextForSource(t, reachModel)
	root, err := ctx.Instantiate(lookupOne(t, idx, "Fuel::vehicle"))
	if err != nil {
		t.Fatal(err)
	}
	var port, line *Instance
	for _, r := range ctx.Traverse(root, TraversalBounds{}).Reaches {
		switch r.Path() {
		case "fuelLine.supplier":
			port = r.Object
		case "fuelLine":
			line = r.Object
		}
	}
	if got := ctx.PathsTo(root, port, PathBounds{Paths: 1}); !got.Truncated || len(got.Steps) != 1 {
		t.Errorf("one path allowed: %d paths, truncated %v", len(got.Steps), got.Truncated)
	}
	if got := ctx.PathsTo(root, port, PathBounds{Depth: 2}); !got.Truncated || pathTexts(got)[0] != "fuelLine.supplier" {
		t.Errorf("two steps allowed: %v, truncated %v", pathTexts(got), got.Truncated)
	}
	if got := ctx.Traverse(root, TraversalBounds{Objects: 3}); !got.Truncated || len(got.Reaches) != 3 {
		t.Errorf("three objects allowed: %d reached, truncated %v", len(got.Reaches), got.Truncated)
	}
	// `fuelLine` is held by the root alone: cutting the deeper branches that
	// never reach it hides no path.
	if got := ctx.PathsTo(root, line, PathBounds{Depth: 1}); got.Truncated || len(got.Steps) != 1 {
		t.Errorf("one step allowed to fuelLine: %v, truncated %v", pathTexts(got), got.Truncated)
	}
	// One step down, `pump` is reached before the tank it refers to is.
	if got := ctx.Traverse(root, TraversalBounds{Depth: 1}); got.Truncated || len(got.Reaches) != 6 {
		t.Errorf("one step allowed: %v, truncated %v", reachPaths(got), got.Truncated)
	}
}

// A recursive type materializes no further for PathsTo than Traverse walks
// it: the root is its own answer with nothing read, and the search below it
// stops where an object's type is already on the path.
func TestPathsToIsBoundedOnARecursiveType(t *testing.T) {
	ctx, idx := contextForSource(t, `package Rec {
	part def Node { part child : Node; attribute n : ScalarValues::Integer; }
	part root : Node;
}`)
	root, err := ctx.Instantiate(lookupOne(t, idx, "Rec::root"))
	if err != nil {
		t.Fatal(err)
	}
	before := ctx.InstanceCount()
	if got := ctx.PathsTo(root, root, PathBounds{Depth: 8}); got.Truncated || len(got.Steps) != 1 || len(got.Steps[0]) != 0 {
		t.Errorf("paths to the root: %v, truncated %v", pathTexts(got), got.Truncated)
	}
	if ctx.InstanceCount() != before {
		t.Errorf("paths to the root materialized %d objects", ctx.InstanceCount()-before)
	}
	reached := ctx.Traverse(root, TraversalBounds{})
	walked := ctx.InstanceCount()
	stranger := &Instance{ID: -1}
	if got := ctx.PathsTo(root, stranger, PathBounds{Depth: 8}); got.Truncated || len(got.Steps) != 0 {
		t.Errorf("paths to an object nothing holds: %v, truncated %v", pathTexts(got), got.Truncated)
	}
	if ctx.InstanceCount() != walked {
		t.Errorf("the search materialized %d objects past Traverse's %d", ctx.InstanceCount()-walked, len(reached.Reaches))
	}
}

// A holder first met beneath a repeated type, where the guard holds its child
// back, is looked into again from the alias that allows the child, so the
// deeper path the alias opens counts as hidden at a cutoff.
func TestPathsToSeesPastAnAliasUnderARepeatedType(t *testing.T) {
	ctx, idx := contextForSource(t, `package Alias {
	part def T;
	part def A { part b : X; }
	part def X { part a : A; part target : T; }
	part root {
		part alpha : X;
		ref part zeta : A = alpha.a;
	}
}`)
	root, err := ctx.Instantiate(lookupOne(t, idx, "Alias::root"))
	if err != nil {
		t.Fatal(err)
	}
	target := heldAt(t, ctx, root, "zeta", "b", "target")
	if got := ctx.PathsTo(root, target, PathBounds{}); strings.Join(pathTexts(got), " ") != "zeta.b.target" || got.Truncated {
		t.Errorf("paths = %v, truncated %v", pathTexts(got), got.Truncated)
	}
	if got := ctx.PathsTo(root, target, PathBounds{Depth: 2}); len(got.Steps) != 0 || !got.Truncated {
		t.Errorf("two steps allowed: %v, truncated %v, want none and truncated", pathTexts(got), got.Truncated)
	}
	// `alpha.a` leads to `b`, which the guard kept the walk out of, but from
	// `alpha.a` it cannot go on: the one path to `alpha.target` is complete.
	other := heldAt(t, ctx, root, "alpha", "target")
	if got := ctx.PathsTo(root, other, PathBounds{Depth: 2}); strings.Join(pathTexts(got), " ") != "alpha.target" || got.Truncated {
		t.Errorf("two steps allowed to alpha.target: %v, truncated %v", pathTexts(got), got.Truncated)
	}
}

// heldAt follows the named features down from root.
func heldAt(t *testing.T, ctx *Context, root *Instance, names ...string) *Instance {
	t.Helper()
	cur := root
	for _, name := range names {
		next := cur
		for _, step := range ctx.heldSteps(cur) {
			if step.Name == name {
				next = step.Object
			}
		}
		if next == cur {
			t.Fatalf("%s holds no %s", cur.Type.Name, name)
		}
		cur = next
	}
	return cur
}

// Deciding what a cutoff hides costs a walk of the graph, not one of its
// routes, however many routes of distinct types share an object, and an
// object the guard keeps the path out of is not looked into for it.
func TestPathsToDecidesTruncationInOneWalk(t *testing.T) {
	const n = 20
	var b strings.Builder
	b.WriteString("package Dense {\n\tpart def T;\n\tpart def A;\n")
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "\tpart def O%d;\n", i)
	}
	b.WriteString("\tpart root {\n\t\tpart target : T;\n\t\tpart a : A {\n\t\t\tpart again : A;\n")
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "\t\t\tpart o%d : O%d {\n", i, i)
		for j := i + 1; j <= n; j++ {
			fmt.Fprintf(&b, "\t\t\t\tref part t%d : O%d = o%d;\n", j, j, j)
		}
		b.WriteString("\t\t\t}\n")
	}
	b.WriteString("\t\t}\n\t}\n}\n")
	ctx, idx := contextForSource(t, b.String())
	root, err := ctx.Instantiate(lookupOne(t, idx, "Dense::root"))
	if err != nil {
		t.Fatal(err)
	}
	if reached := ctx.Traverse(root, TraversalBounds{}); len(reached.Reaches) != n+4 {
		t.Fatalf("reached %d objects, want %d: %v", len(reached.Reaches), n+4, reachPaths(reached))
	}
	target := heldAt(t, ctx, root, "target")
	if got := ctx.PathsTo(root, target, PathBounds{Depth: 1}); strings.Join(pathTexts(got), " ") != "target" || got.Truncated {
		t.Errorf("one step allowed to target: %v, truncated %v", pathTexts(got), got.Truncated)
	}
	if got := ctx.PathsTo(root, &Instance{ID: -1}, PathBounds{Depth: 1}); got.Truncated || len(got.Steps) != 0 {
		t.Errorf("paths to an object nothing holds: %v, truncated %v", pathTexts(got), got.Truncated)
	}
}

// A cutoff above a recursive type looks into none of it: the only routes to
// the objects the guard kept the walk out of pass an object of their type.
func TestPathsToKeepsOutOfARecursiveTypeBelowACutoff(t *testing.T) {
	ctx, idx := contextForSource(t, `package Below {
	part def L;
	part def A { part child : A; part leaf : L; }
	part def X { part a : A; }
	part root { part x : X; part target : L; }
}`)
	root, err := ctx.Instantiate(lookupOne(t, idx, "Below::root"))
	if err != nil {
		t.Fatal(err)
	}
	reached := ctx.Traverse(root, TraversalBounds{})
	walked := ctx.InstanceCount()
	target := heldAt(t, ctx, root, "target")
	if got := ctx.PathsTo(root, target, PathBounds{Depth: 1}); strings.Join(pathTexts(got), " ") != "target" || got.Truncated {
		t.Errorf("one step allowed to target: %v, truncated %v", pathTexts(got), got.Truncated)
	}
	if got := ctx.PathsTo(root, &Instance{ID: -1}, PathBounds{Depth: 1}); got.Truncated || len(got.Steps) != 0 {
		t.Errorf("paths to an object nothing holds: %v, truncated %v", pathTexts(got), got.Truncated)
	}
	if ctx.InstanceCount() != walked {
		t.Errorf("the cutoff materialized %d objects past Traverse's %d", ctx.InstanceCount()-walked, len(reached.Reaches))
	}
}
