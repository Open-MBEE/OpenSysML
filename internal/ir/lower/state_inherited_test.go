package lower

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A state usage typed by a state definition owns that definition's substates,
// not an empty body.
func TestToStateGraphTypedStateUsageInheritsSubstates(t *testing.T) {
	graph := stateGraphOf(t, `
		package test {
			state def Inner {
				entry; then i1;
				state i1;
				state i2;
			}
			state def Machine {
				entry; then nested;
				state nested : Inner;
			}
		}
	`, "Machine")

	nested := stateNamed(graph, "nested")
	if nested == nil {
		t.Fatal("typed state usage was not collected")
	}
	for _, name := range []string{"i1", "i2"} {
		child := stateNamed(graph, name)
		if child == nil {
			t.Fatalf("inherited substate %s was not collected", name)
		}
		if graph.ParentState[child] != nested {
			t.Fatalf("parent of %s = %v, want nested", name, graph.ParentState[child])
		}
	}
	if !graph.IsInitial(stateNamed(graph, "i1")) {
		t.Fatal("the inherited initial transition did not reach the usage")
	}
}

// Two usages of one definition are two materializations: separate vertices and
// separate attributes, so neither aliases the other at runtime.
func TestToStateGraphTwoTypedUsagesAreIndependent(t *testing.T) {
	graph := stateGraphOf(t, `
		package test {
			state def Inner {
				attribute hits;
				entry; then i1;
				state i1;
			}
			state def Machine {
				entry; then one;
				state one : Inner;
				state two : Inner;
			}
		}
	`, "Machine")

	one, two := stateNamed(graph, "one"), stateNamed(graph, "two")
	if one == nil || two == nil {
		t.Fatal("both typed usages must be collected")
	}
	if one == two {
		t.Fatal("two usages of one definition share a vertex")
	}
	var leaves []*ast.StateNode
	for _, state := range graph.States {
		if state.Name == "i1" {
			leaves = append(leaves, state)
		}
	}
	if len(leaves) != 2 {
		t.Fatalf("inherited leaves = %d, want one per usage", len(leaves))
	}
	if leaves[0] == leaves[1] {
		t.Fatal("both usages share the inherited leaf")
	}
	if len(graph.StateAttributes[one]) != 1 || len(graph.StateAttributes[two]) != 1 {
		t.Fatalf("attributes: one = %v, two = %v, want one each",
			graph.StateAttributes[one], graph.StateAttributes[two])
	}
}

// Inheritance composes: a definition whose substate is itself a typed usage
// reaches the innermost definition's content.
func TestToStateGraphInheritanceTwoLevelsDeep(t *testing.T) {
	graph := stateGraphOf(t, `
		package test {
			state def Leaf {
				entry; then l1;
				state l1;
			}
			state def Middle {
				entry; then deep;
				state deep : Leaf;
			}
			state def Machine {
				entry; then mid;
				state mid : Middle;
			}
		}
	`, "Machine")

	leaf := stateNamed(graph, "l1")
	if leaf == nil {
		t.Fatal("two-level inherited substate was not collected")
	}
	deep := graph.ParentState[leaf]
	if deep == nil || deep.Name != "deep" {
		t.Fatalf("parent of l1 = %v, want deep", deep)
	}
	if parent := graph.ParentState[deep]; parent == nil || parent.Name != "mid" {
		t.Fatalf("parent of deep = %v, want mid", parent)
	}
}

// A usage's own body adds to what it inherits and redeclares what it names
// again: the inherited substate of that name is the one the usage writes.
func TestToStateGraphUsageBodyAddsAndRedeclares(t *testing.T) {
	graph := stateGraphOf(t, `
		package test {
			state def Inner {
				entry; then i1;
				state i1;
				state i2;
			}
			state def Machine {
				entry; then nested;
				state nested : Inner {
					state i2 { entry action own; }
					state i3;
				}
			}
		}
	`, "Machine")

	nested := stateNamed(graph, "nested")
	var names []string
	var redeclared *ast.StateNode
	for _, state := range graph.States {
		if graph.ParentState[state] != nested {
			continue
		}
		names = append(names, state.Name)
		if state.Name == "i2" {
			redeclared = state
		}
	}
	if len(names) != 3 {
		t.Fatalf("substates of nested = %v, want i1, i2 and i3", names)
	}
	if redeclared == nil || len(graph.Behaviors[redeclared].Entry) != 1 {
		t.Fatalf("the usage's own i2 did not replace the inherited one: %v", redeclared)
	}
}

// A typed usage in a parallel body is an orthogonal region entered at the
// initial state its definition declares.
func TestToStateGraphTypedParallelRegion(t *testing.T) {
	graph := stateGraphOf(t, `
		package test {
			state def Region {
				entry; then r1;
				state r1;
			}
			state def Machine {
				entry; then running;
				state running parallel {
					state a : Region;
					state b : Region;
				}
			}
		}
	`, "Machine")

	running := stateNamed(graph, "running")
	if running == nil || len(running.Regions) != 2 {
		t.Fatalf("regions of running = %v, want one per typed usage", running)
	}
	for _, region := range running.Regions {
		if graph.RegionInitials[region] == nil {
			t.Fatalf("region %s has no inherited initial state", region.Name)
		}
	}
}

// A definition reaching itself through its own content has no finite
// materialization, and lowering says so.
func TestToStateGraphRecursiveTypingIsAnError(t *testing.T) {
	_, err := stateGraphErr(t, `
		package test {
			state def A {
				entry; then b;
				state b : A;
			}
		}
	`, "A")
	if !errors.Is(err, ErrRecursiveStateTyping) {
		t.Fatalf("error = %v, want recursive state typing", err)
	}
}

// Inherited content lowering cannot represent surfaces as a typed error rather
// than disappearing from the usage.
func TestToStateGraphUnsupportedInheritedMemberIsAnError(t *testing.T) {
	_, err := stateGraphErr(t, `
		package test {
			action def Warm;
			state def Inner {
				entry; then i1;
				state i1;
				perform Warm;
			}
			state def Machine {
				entry; then nested;
				state nested : Inner;
			}
		}
	`, "Machine")
	if !errors.Is(err, ErrUnsupportedStateContent) {
		t.Fatalf("error = %v, want unsupported state content", err)
	}
}

// stateGraphOf lowers the named state definition in src, which must succeed.
func stateGraphOf(t *testing.T, src, name string) *StateGraph {
	t.Helper()
	graph, err := stateGraphErr(t, src, name)
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	return graph
}

// stateGraphErr lowers the named state definition in src with the scope tree its
// type names resolve in.
func stateGraphErr(t *testing.T, src, name string) (*StateGraph, error) {
	t.Helper()
	p := parser.New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc("test.sysml", root)
	decl := stateDefinition(t, root, name)
	scope := scopeOfDecl(idx.DocumentRoot("test.sysml"), decl)
	if scope == nil {
		t.Fatalf("scope for %s missing", name)
	}
	return ToStateGraph(decl, scope)
}

// scopeOfDecl finds the scope built for a declaration anywhere under root.
func scopeOfDecl(root *symbols.Scope, decl ast.Node) *symbols.Scope {
	if root == nil {
		return nil
	}
	if scope := root.ChildFor(decl); scope != nil {
		return scope
	}
	for _, child := range root.Children() {
		if scope := scopeOfDecl(child, decl); scope != nil {
			return scope
		}
	}
	return nil
}

// stateDefinition finds the state definition named name in root.
func stateDefinition(t *testing.T, root *ast.RootNamespace, name string) *ast.Definition {
	t.Helper()
	var find func([]ast.Node) *ast.Definition
	find = func(members []ast.Node) *ast.Definition {
		for _, member := range members {
			switch n := unwrapMembership(member).(type) {
			case *ast.Definition:
				if n.Kind == ast.DefState && n.Ident.Name == name {
					return n
				}
				if found := find(n.Members); found != nil {
					return found
				}
			case *ast.Package:
				if found := find(n.Members); found != nil {
					return found
				}
			}
		}
		return nil
	}
	decl := find(root.Members)
	if decl == nil {
		t.Fatalf("state definition %s not found", name)
	}
	return decl
}

// A machine specializing a state definition may also use that definition for a
// substate: the reuse is not recursion.
func TestToStateGraphMachineSupertypeReusedBySubstate(t *testing.T) {
	graph := stateGraphOf(t, `
		package test {
			state def Inner {
				entry; then i1;
				state i1;
			}
			state def Machine :> Inner {
				state sibling : Inner;
			}
		}
	`, "Machine")

	sibling := stateNamed(graph, "sibling")
	if sibling == nil {
		t.Fatal("the substate typed by the machine's own supertype was not collected")
	}
	var leaves int
	for _, state := range graph.States {
		if state.Name == "i1" {
			leaves++
		}
	}
	if leaves != 2 {
		t.Fatalf("i1 vertices = %d, want one inherited by the machine and one under sibling", leaves)
	}
}

func TestToStateGraphUsageRedeclaresInheritedSubstate(t *testing.T) {
	graph := stateGraphOf(t, `
		package test {
			state def Base {
				entry; then working;
				state working;
			}
			state def Machine {
				entry; then u;
				state u : Base {
					state working { state deeper; }
				}
			}
		}
	`, "Machine")

	var working *ast.StateNode
	for _, state := range graph.States {
		if state.Name != "working" {
			continue
		}
		if working != nil {
			t.Fatal("the redeclared substate was collected twice")
		}
		working = state
	}
	if working == nil {
		t.Fatal("no working state was collected")
	}
	if stateNamed(graph, "deeper") == nil {
		t.Fatal("the replacement substate's own body was not collected")
	}
	if !graph.IsInitial(working) {
		t.Fatal("the inherited initial transition did not reach the redeclared substate")
	}
}

// A transition a usage inherits ends at the usage's own copy of the terminate
// action or pseudostate it names, never at the definition's declaration, so two
// usages of one definition terminate or route through two distinct vertices —
// the copy nested in a composite the usage inherits included.
func TestToStateGraphTwoTypedUsagesOwnInheritedTerminateAndPseudostate(t *testing.T) {
	graph := stateGraphOf(t, `
		package test {
			attribute def Abort;
			attribute def Check;
			state def Inner {
				entry; then idle;
				state idle;
				transition first idle accept Abort then stop;
				action stop terminate;
				transition first idle accept Check then relay;
				junction relay;
				transition first relay then work;
				state work {
					entry; then w1;
					state w1;
					transition first w1 accept Abort then halt;
					action halt terminate;
				}
			}
			state def Machine {
				entry; then one;
				state one : Inner;
				state two : Inner;
			}
		}
	`, "Machine")

	one, two := stateNamed(graph, "one"), stateNamed(graph, "two")
	if one == nil || two == nil {
		t.Fatal("both typed usages must be collected")
	}
	if len(graph.Terminates) != 4 {
		t.Fatalf("terminate vertices = %d, want two per usage", len(graph.Terminates))
	}
	if len(graph.Pseudostates) != 2 {
		t.Fatalf("pseudostates = %d, want one per usage", len(graph.Pseudostates))
	}
	ownerOf := func(usage *ast.StateNode, target ast.Node) *ast.StateNode {
		switch v := target.(type) {
		case *ast.Usage:
			return graph.TerminateOwner[v]
		case *ast.PseudostateNode:
			return graph.PseudostateOwner[v]
		}
		return nil
	}
	targets := map[ast.Node]bool{}
	for _, usage := range []*ast.StateNode{one, two} {
		for _, name := range []string{"stop", "relay"} {
			target := inheritedTarget(t, graph, usage, name)
			if targets[target] {
				t.Fatalf("both usages share the inherited %s vertex", name)
			}
			targets[target] = true
			if owner := ownerOf(usage, target); owner != usage {
				t.Fatalf("owner of %s under %s = %v, want the usage itself", name, usage.Name, owner)
			}
		}
		halt := inheritedTarget(t, graph, usage, "halt")
		if targets[halt] {
			t.Fatal("both usages share the terminate action nested in the inherited composite")
		}
		targets[halt] = true
		if owner := ownerOf(usage, halt); owner == nil || owner.Name != "work" || graph.ParentState[owner] != usage {
			t.Fatalf("owner of halt under %s = %v, want that usage's copy of work", usage.Name, owner)
		}
	}
}

// inheritedTarget is the vertex the transitions under usage end at when they name
// the inherited vertex called name.
func inheritedTarget(t *testing.T, graph *StateGraph, usage *ast.StateNode, name string) ast.Node {
	t.Helper()
	for _, source := range graph.States {
		for p := source; p != nil; p = graph.ParentState[p] {
			if p != usage {
				continue
			}
			for _, trans := range graph.Transitions[source] {
				if vertexName(trans.Target) == name {
					return trans.Target
				}
			}
		}
	}
	t.Fatalf("no transition under %s ends at %s", usage.Name, name)
	return nil
}
