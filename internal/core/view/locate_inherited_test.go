package view

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// inheritedBase declares the machine inheritedDerived specializes, in a document
// of its own.
const inheritedBase = `package Plant {
    attribute def Go;
    state def Machine {
        entry; then idle;
        state idle;
        transition idle_to_halted first idle accept Go then halted;
        state halted;
    }
}
`

// inheritedDerived specializes the machine of inheritedBase and exposes it.
const inheritedDerived = `package Derived {
    private import Plant::*;
    private import StandardViewDefinitions::*;
    state def Special :> Plant::Machine {
        state extra;
        transition first halted then extra;
    }
    view specialStates : StateTransitionView { expose Derived::Special; }
}
`

// inheritedFlow declares an action after the one it takes nodes from, with a
// marker between them where an edit moves the specializing action alone.
const inheritedFlow = `package Flows {
    private import StandardViewDefinitions::*;
    action def Flow {
        action a;
        action b;
    }
    // edit here
    action def SpecialFlow :> Flow {
        first start;
        then a;
        succession a then b;
        succession b then done;
    }
    view specialFlow : ActionFlowView { expose Flows::SpecialFlow; }
}
`

const inheritedInsertion = "part def Filler;\n    part def Spacer;\n"

// renderModel renders view over the named documents and returns the rendering,
// behavior's symbol and the renderer, whose resolver lowering shares.
func renderModel(t *testing.T, names, texts []string, view, behavior string) (*Rendering, *symbols.Symbol, *Renderer) {
	t.Helper()
	contents := make([][]byte, len(texts))
	for i, text := range texts {
		contents[i] = []byte(text)
	}
	r, idx := loadSources(t, names, contents)
	rendering, err := r.Render(lookup(t, idx, view))
	if err != nil {
		t.Fatalf("render %s: %v", view, err)
	}
	return rendering, lookup(t, idx, behavior), r
}

// nodeDoc is the document the rendered node with the given ID was drawn from.
func nodeDoc(t *testing.T, rendering *Rendering, id string) string {
	t.Helper()
	for _, node := range rendering.Data().Nodes {
		if node.ID == id {
			return node.Origin.Doc
		}
	}
	t.Fatalf("rendering has no node %q", id)
	return ""
}

// States and transitions a machine inherits from a definition in another document
// are drawn as written there, and a locator still finds them in a rendering made
// after an edit ahead of that definition moved every span in its document.
func TestLocateStatesInheritedFromAnotherDocument(t *testing.T) {
	docs := []string{"base.sysml", "derived.sysml"}
	before, sym, r := renderModel(t, docs, []string{inheritedBase, inheritedDerived}, "Derived::specialStates", "Derived::Special")
	graph, err := lower.ToStateGraphWithEndpoints(sym.Decl, declScope(sym), lower.NewLibraryStateTypes(r.resolver))
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	if len(graph.Inherited()) != 1 {
		t.Fatalf("graph inherits from %d declarations, want Plant::Machine alone", len(graph.Inherited()))
	}
	loc, err := LocateStates(before, sym, sym, graph)
	if err != nil {
		t.Fatalf("LocateStates: %v", err)
	}
	id, ok := loc.Node(stateNamed(t, graph, "idle"))
	if !ok {
		t.Fatal("Node(idle) not located before the edit")
	}
	if doc := nodeDoc(t, before, id); doc != "base.sysml" {
		t.Errorf("inherited state idle drawn from %q, want base.sysml", doc)
	}

	after, drawn, _ := renderModel(t, docs, []string{inheritedInsertion + inheritedBase, inheritedDerived}, "Derived::specialStates", "Derived::Special")
	if drawn.DeclSpan != sym.DeclSpan {
		t.Fatal("an edit to the base document moved the derived machine")
	}
	loc, err = LocateStates(after, drawn, sym, graph)
	if err != nil {
		t.Fatalf("LocateStates after the edit: %v", err)
	}
	for _, name := range []string{"idle", "halted", "extra"} {
		id, ok := loc.Node(stateNamed(t, graph, name))
		if !ok {
			t.Errorf("Node(%s) not located after the edit", name)
			continue
		}
		if got := nodeName(t, after, id); got != name {
			t.Errorf("Node(%s) draws %q after the edit", name, got)
		}
	}
	for _, ends := range [][2]string{{"idle", "halted"}, {"halted", "extra"}} {
		transition := transitionFrom(t, graph, ends[0], ends[1])
		index, ok := loc.Transition(transition.Decl, transition.Source, transition.Target)
		if !ok {
			t.Errorf("Transition(%s -> %s) not located after the edit", ends[0], ends[1])
			continue
		}
		if got := edgeEnds(t, after, index); got != ends {
			t.Errorf("Transition(%s -> %s) draws %v after the edit", ends[0], ends[1], got)
		}
	}
	entries := graph.EntryTransitions[nil]
	if len(entries) != 1 {
		t.Fatalf("machine entry transitions = %v, want the inherited one", entries)
	}
	index, ok := loc.EntryTransition(entries[0].Decl, nil, entries[0].Target)
	if !ok {
		t.Fatal("EntryTransition(-> idle) not located after the edit")
	}
	if start, _ := loc.Start(nil); after.Data().Edges[index].From != start || edgeEnds(t, after, index)[1] != "idle" {
		t.Errorf("EntryTransition(-> idle) draws %v after the edit", edgeEnds(t, after, index))
	}
}

// Nodes an action inherits keep the positions of the definition they were written
// in, so an edit between that definition and the action, which moves the action
// alone, still leaves every node and succession located.
func TestLocateActionsInheritedAcrossAnEdit(t *testing.T) {
	docs := []string{"flows.sysml"}
	before, sym, _ := renderModel(t, docs, []string{inheritedFlow}, "Flows::specialFlow", "Flows::SpecialFlow")
	graph, err := lower.ToActionGraph(sym.Decl, declScope(sym))
	if err != nil {
		t.Fatalf("ToActionGraph: %v", err)
	}
	if len(graph.Inherited()) != 1 {
		t.Fatalf("graph inherits from %d declarations, want Flows::Flow alone", len(graph.Inherited()))
	}
	if _, err := LocateActions(before, sym, sym, graph); err != nil {
		t.Fatalf("LocateActions: %v", err)
	}

	edited := strings.Replace(inheritedFlow, "// edit here\n", inheritedInsertion, 1)
	after, drawn, _ := renderModel(t, docs, []string{edited}, "Flows::specialFlow", "Flows::SpecialFlow")
	if drawn.DeclSpan.Offset == sym.DeclSpan.Offset {
		t.Fatal("the edit did not move the specializing action")
	}
	loc, err := LocateActions(after, drawn, sym, graph)
	if err != nil {
		t.Fatalf("LocateActions after the edit: %v", err)
	}
	for _, name := range []string{"start", "a", "b", "done"} {
		id, ok := loc.Node(nil, actionNamed(t, graph, name))
		if !ok {
			t.Errorf("Node(%s) not located after the edit", name)
			continue
		}
		if got := nodeName(t, after, id); got != name {
			t.Errorf("Node(%s) draws %q after the edit", name, got)
		}
	}
	for _, ends := range [][2]string{{"start", "a"}, {"a", "b"}, {"b", "done"}} {
		edge := successionFrom(t, graph, ends[0], ends[1])
		index, ok := loc.Edge(nil, edge)
		if !ok {
			t.Errorf("Edge(%s -> %s) not located after the edit", ends[0], ends[1])
			continue
		}
		if got := edgeEnds(t, after, index); got != ends {
			t.Errorf("Edge(%s -> %s) draws %v after the edit", ends[0], ends[1], got)
		}
	}
}
