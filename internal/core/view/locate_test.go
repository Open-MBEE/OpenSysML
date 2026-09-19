package view

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// locatorModel renders view from the fixture with prefix inserted before the
// model text, and returns the rendering with the symbol of behavior in it.
func locatorModel(t *testing.T, file, prefix, view, behavior string) (*Rendering, *symbols.Symbol) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", file))
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	r, idx := loadSources(t, []string{file}, [][]byte{append([]byte(prefix), content...)})
	rendering, err := r.Render(lookup(t, idx, view))
	if err != nil {
		t.Fatalf("render %s: %v", view, err)
	}
	return rendering, lookup(t, idx, behavior)
}

// nodeName is the name of the rendered node with the given ID.
func nodeName(t *testing.T, rendering *Rendering, id string) string {
	t.Helper()
	for _, node := range rendering.Data().Nodes {
		if node.ID == id {
			return node.Name
		}
	}
	t.Fatalf("rendering has no node %q", id)
	return ""
}

// edgeEnds are the names of the nodes the rendered edge at index joins.
func edgeEnds(t *testing.T, rendering *Rendering, index int) [2]string {
	t.Helper()
	edges := rendering.Data().Edges
	if index < 0 || index >= len(edges) {
		t.Fatalf("edge %d out of range of %d edges", index, len(edges))
	}
	return [2]string{nodeName(t, rendering, edges[index].From), nodeName(t, rendering, edges[index].To)}
}

func stateNamed(t *testing.T, graph *lower.StateGraph, name string) *ast.StateNode {
	t.Helper()
	for _, state := range graph.States {
		if state.Name == name {
			return state
		}
	}
	t.Fatalf("graph has no state %q", name)
	return nil
}

func transitionFrom(t *testing.T, graph *lower.StateGraph, source, target string) *lower.Transition {
	t.Helper()
	for _, tr := range graph.Transitions[stateNamed(t, graph, source)] {
		if state, ok := tr.Target.(*ast.StateNode); ok && state.Name == target {
			return tr
		}
	}
	t.Fatalf("graph has no transition %s -> %s", source, target)
	return nil
}

// The state locator maps lowered vertices and transitions to the nodes and edges
// drawing them, entry transitions leaving their body's start marker.
func TestLocateStatesMapsVerticesAndTransitions(t *testing.T) {
	rendering, sym := locatorModel(t, "state.sysml", "", "MachineViews::vehicleStates", "Machines::VehicleStates")
	graph, err := lower.ToStateGraph(sym.Decl, declScope(sym))
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	loc, err := LocateStates(rendering, sym, sym, graph)
	if err != nil {
		t.Fatalf("LocateStates: %v", err)
	}
	if got := nodeName(t, rendering, loc.Root()); got != "Machines::VehicleStates" {
		t.Errorf("Root() draws %q, want Machines::VehicleStates", got)
	}
	for _, name := range []string{"off", "operating", "idle", "moving"} {
		id, ok := loc.Node(stateNamed(t, graph, name))
		if !ok {
			t.Errorf("Node(%s) not located", name)
			continue
		}
		if got := nodeName(t, rendering, id); got != name {
			t.Errorf("Node(%s) draws %q", name, got)
		}
	}
	tr := transitionFrom(t, graph, "idle", "moving")
	index, ok := loc.Transition(tr.Decl, tr.Source, tr.Target)
	if !ok {
		t.Fatal("Transition(idle -> moving) not located")
	}
	if got := edgeEnds(t, rendering, index); got != [2]string{"idle", "moving"} {
		t.Errorf("Transition(idle -> moving) draws %v", got)
	}
	entry := graph.EntryTransitions[stateNamed(t, graph, "operating")]
	if len(entry) != 1 {
		t.Fatalf("operating has %d entry transitions, want 1", len(entry))
	}
	operating := stateNamed(t, graph, "operating")
	if _, ok := loc.EntryTransition(entry[0].Decl, operating, entry[0].Target); !ok {
		t.Error("entry transition of operating not located")
	}
	if _, ok := loc.Start(operating); !ok {
		t.Error("Start(operating) not located")
	}
}

// An entry transition written in one body may start the machine in a state nested
// deeper; the edge leaves the start marker of the body it is written in.
func TestLocateStatesEntryTransitionIntoNestedState(t *testing.T) {
	const model = `package Machines {
	state def Deep {
		entry; then working::step1;
		state working {
			state step1;
			state step2;
			transition first step1 then step2;
		}
		state done;
		transition first working then done;
	}
}
package MachineViews {
	private import StandardViewDefinitions::*;
	view deepStates : StateTransitionView { expose Machines::Deep; }
}
`
	r, idx := loadSources(t, []string{"deep.sysml"}, [][]byte{[]byte(model)})
	rendering, err := r.Render(lookup(t, idx, "MachineViews::deepStates"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	sym := lookup(t, idx, "Machines::Deep")
	graph, err := lower.ToStateGraph(sym.Decl, declScope(sym))
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	entry := graph.EntryTransitions[nil]
	if len(entry) != 1 || entry[0].Target != stateNamed(t, graph, "step1") {
		t.Fatalf("machine entry transitions = %v, want one to step1", entry)
	}
	loc, err := LocateStates(rendering, sym, sym, graph)
	if err != nil {
		t.Fatalf("LocateStates: %v", err)
	}
	index, ok := loc.EntryTransition(entry[0].Decl, nil, entry[0].Target)
	if !ok {
		t.Fatal("EntryTransition(machine -> step1) not located")
	}
	edge := rendering.Data().Edges[index]
	if start, _ := loc.Start(nil); edge.From != start || nodeName(t, rendering, edge.To) != "step1" {
		t.Errorf("EntryTransition(machine -> step1) draws %s -> %s, want the machine's start -> step1", edge.From, edge.To)
	}
	if _, ok := loc.EntryTransition(entry[0].Decl, stateNamed(t, graph, "working"), entry[0].Target); ok {
		t.Error("EntryTransition located from working's start, which draws no such edge")
	}
}

// Declarations inserted ahead of the machine shift every offset, yet a locator on
// the fresh rendering still finds a graph lowered from the earlier text.
func TestLocateStatesSurvivesShiftedOffsets(t *testing.T) {
	_, sym := locatorModel(t, "state.sysml", "", "MachineViews::vehicleStates", "Machines::VehicleStates")
	graph, err := lower.ToStateGraph(sym.Decl, declScope(sym))
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	prefix := "package Inserted { part def Filler; }\n\n"
	after, drawn := locatorModel(t, "state.sysml", prefix, "MachineViews::vehicleStates", "Machines::VehicleStates")
	if drawn.DeclSpan.Offset == sym.DeclSpan.Offset {
		t.Fatal("prefix did not shift the machine declaration")
	}
	if _, err := LocateStates(after, sym, sym, graph); !errors.Is(err, ErrNotDrawn) {
		t.Errorf("locating by the stale symbol err = %v, want ErrNotDrawn", err)
	}
	loc, err := LocateStates(after, drawn, sym, graph)
	if err != nil {
		t.Fatalf("LocateStates: %v", err)
	}
	for _, name := range []string{"off", "operating", "idle", "moving"} {
		id, ok := loc.Node(stateNamed(t, graph, name))
		if !ok {
			t.Errorf("Node(%s) not located after shift", name)
			continue
		}
		if got := nodeName(t, after, id); got != name {
			t.Errorf("Node(%s) draws %q after shift", name, got)
		}
	}
	tr := transitionFrom(t, graph, "operating", "off")
	index, ok := loc.Transition(tr.Decl, tr.Source, tr.Target)
	if !ok {
		t.Fatal("Transition(operating -> off) not located after shift")
	}
	if got := edgeEnds(t, after, index); got != [2]string{"operating", "off"} {
		t.Errorf("Transition(operating -> off) draws %v after shift", got)
	}
}

func actionNamed(t *testing.T, graph *lower.ActionGraph, name string) ast.Node {
	t.Helper()
	for _, node := range graph.Nodes {
		if behaviorNodeName(node) == name {
			return node
		}
	}
	t.Fatalf("graph has no node %q", name)
	return nil
}

func successionFrom(t *testing.T, graph *lower.ActionGraph, source, target string) lower.ActionEdge {
	t.Helper()
	for _, edge := range graph.Edges[actionNamed(t, graph, source)] {
		if behaviorNodeName(edge.Target) == target {
			return edge
		}
	}
	t.Fatalf("graph has no succession %s -> %s", source, target)
	return lower.ActionEdge{}
}

// The action locator maps lowered nodes and successions, nested flows included,
// and places an undrawn node at the innermost drawn node around it.
func TestLocateActionsMapsNodesAndNestedFlows(t *testing.T) {
	rendering, sym := locatorModel(t, "action.sysml", "", "FlowViews::driveView", "Flows::Drive")
	graph, err := lower.ToActionGraph(sym.Decl, declScope(sym))
	if err != nil {
		t.Fatalf("ToActionGraph: %v", err)
	}
	loc, err := LocateActions(rendering, sym, sym, graph)
	if err != nil {
		t.Fatalf("LocateActions: %v", err)
	}
	if got := nodeName(t, rendering, loc.Root()); got != "Flows::Drive" {
		t.Errorf("Root() draws %q, want Flows::Drive", got)
	}
	for _, name := range []string{"start", "split", "provide", "monitor", "tally", "sync", "check", "park"} {
		id, ok := loc.Node(nil, actionNamed(t, graph, name))
		if !ok {
			t.Errorf("Node(%s) not located", name)
			continue
		}
		if got := nodeName(t, rendering, id); got != name {
			t.Errorf("Node(%s) draws %q", name, got)
		}
	}
	edge := successionFrom(t, graph, "split", "tally")
	index, ok := loc.Edge(nil, edge)
	if !ok {
		t.Fatal("Edge(split -> tally) not located")
	}
	if got := edgeEnds(t, rendering, index); got != [2]string{"split", "tally"} {
		t.Errorf("Edge(split -> tally) draws %v", got)
	}

	monitor := actionNamed(t, graph, "monitor")
	nested, err := lower.ToActionGraph(monitor, declScope(sym))
	if err != nil {
		t.Fatalf("ToActionGraph(monitor): %v", err)
	}
	within := []ast.Node{monitor}
	record := actionNamed(t, nested, "record")
	id, ok := loc.Node(within, record)
	if !ok {
		t.Fatal("Node(monitor/record) not located")
	}
	if got := nodeName(t, rendering, id); got != "record" {
		t.Errorf("Node(monitor/record) draws %q", got)
	}
	inner := successionFrom(t, nested, "begin", "record")
	index, ok = loc.Edge(within, inner)
	if !ok {
		t.Fatal("Edge(monitor/begin -> record) not located")
	}
	if got := edgeEnds(t, rendering, index); got != [2]string{"begin", "record"} {
		t.Errorf("Edge(monitor/begin -> record) draws %v", got)
	}

	// A node inside a nesting the rendering does not draw falls back to the
	// innermost drawn node, reported as not located.
	id, ok = loc.Node([]ast.Node{monitor, record}, nil)
	if ok {
		t.Error("Node(monitor/record/...) reported located")
	}
	if got := nodeName(t, rendering, id); got != "record" {
		t.Errorf("Node(monitor/record/...) placed at %q, want record", got)
	}
}

// As for machines, an action locator built from a shifted rendering finds the
// nodes of the graph lowered from the earlier text.
func TestLocateActionsSurvivesShiftedOffsets(t *testing.T) {
	_, sym := locatorModel(t, "action.sysml", "", "FlowViews::driveView", "Flows::Drive")
	graph, err := lower.ToActionGraph(sym.Decl, declScope(sym))
	if err != nil {
		t.Fatalf("ToActionGraph: %v", err)
	}
	after, drawn := locatorModel(t, "action.sysml", "// header\n\n", "FlowViews::driveView", "Flows::Drive")
	loc, err := LocateActions(after, drawn, sym, graph)
	if err != nil {
		t.Fatalf("LocateActions: %v", err)
	}
	edge := successionFrom(t, graph, "tally", "sync")
	index, ok := loc.Edge(nil, edge)
	if !ok {
		t.Fatal("Edge(tally -> sync) not located after shift")
	}
	if got := edgeEnds(t, after, index); got != [2]string{"tally", "sync"} {
		t.Errorf("Edge(tally -> sync) draws %v after shift", got)
	}
}
