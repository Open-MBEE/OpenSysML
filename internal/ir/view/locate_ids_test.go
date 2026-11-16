package view

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// The state locator answers by id what it answers by declaration: every state,
// transition, start marker and entry transition of the machine, and nothing for
// an id the graph does not hold.
func TestLocateStatesByIDAgreesWithDeclarations(t *testing.T) {
	rendering, sym := locatorModel(t, "state.sysml", "", "MachineViews::vehicleStates", "Machines::VehicleStates")
	graph, err := lower.ToStateGraph(sym.Decl, declScope(sym))
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	loc, err := LocateStates(rendering, sym, sym, graph)
	if err != nil {
		t.Fatalf("LocateStates: %v", err)
	}
	located, transitions, entries := 0, 0, 0
	for _, id := range graph.StateIDs() {
		state := graph.Provenance.State(id)
		want, wantOK := loc.Node(state)
		got, ok := loc.Vertex(program.StateVertex(id))
		if got != want || ok != wantOK {
			t.Errorf("Vertex(%s) = %q, %v; Node = %q, %v", state.Name, got, ok, want, wantOK)
		}
		if ok {
			located++
		}
		wantStart, wantOK := loc.Start(state)
		start, ok := loc.StateStart(id)
		if start != wantStart || ok != wantOK {
			t.Errorf("StateStart(%s) = %q, %v; Start = %q, %v", state.Name, start, ok, wantStart, wantOK)
		}
		for _, tr := range graph.TransitionsOf(program.StateVertex(id)) {
			want, wantOK := loc.Transition(tr.Decl, tr.Source, tr.Target)
			got, ok := loc.TransitionEdge(tr)
			if got != want || ok != wantOK || !ok {
				t.Errorf("TransitionEdge(%s) = %d, %v; Transition = %d, %v", state.Name, got, ok, want, wantOK)
			}
			transitions++
		}
		for _, entry := range graph.StartOfState(id) {
			want, wantOK := loc.EntryTransition(entry.Decl, state, entry.Target)
			got, ok := loc.EntryEdge(entry, start)
			if got != want || ok != wantOK || !ok {
				t.Errorf("EntryEdge(%s) = %d, %v; EntryTransition = %d, %v", state.Name, got, ok, want, wantOK)
			}
			entries++
		}
	}
	if located != 4 || transitions != 4 || entries != 1 {
		t.Errorf("located %d states, %d transitions, %d entry transitions; want 4, 4, 1", located, transitions, entries)
	}

	wantStart, wantOK := loc.Start(nil)
	start, ok := loc.StateStart(0)
	if start != wantStart || ok != wantOK || !ok {
		t.Errorf("StateStart(0) = %q, %v; Start(nil) = %q, %v", start, ok, wantStart, wantOK)
	}
	machine := graph.StartOfState(0)
	if len(machine) != 1 {
		t.Fatalf("the machine has %d entry transitions, want 1", len(machine))
	}
	index, ok := loc.EntryEdge(machine[0], start)
	want, wantOK := loc.EntryTransition(machine[0].Decl, nil, machine[0].Target)
	if index != want || ok != wantOK || !ok {
		t.Errorf("EntryEdge(machine) = %d, %v; EntryTransition = %d, %v", index, ok, want, wantOK)
	}
	if edge := rendering.Data().Edges[index]; edge.From != start || nodeName(t, rendering, edge.To) != "off" {
		t.Errorf("EntryEdge(machine) draws %s -> %s, want the machine's start -> off", edge.From, edge.To)
	}

	unknown := program.StateID(len(graph.StateIDs()) + 100)
	if id, ok := loc.Vertex(program.StateVertex(unknown)); ok {
		t.Errorf("Vertex of an id the graph does not hold located at %q", id)
	}
	if id, ok := loc.StateStart(unknown); ok {
		t.Errorf("StateStart of an id the graph does not hold located at %q", id)
	}
	if id, ok := loc.Region(1); ok {
		t.Errorf("Region(1) located at %q in a machine with no regions", id)
	}
	if id, ok := loc.RegionStart(1); ok {
		t.Errorf("RegionStart(1) located at %q in a machine with no regions", id)
	}
}

// The action locator answers by id what it answers by declaration, in the
// action's own flow and in the flow of a nested action.
func TestLocateActionsByIDAgreesWithDeclarations(t *testing.T) {
	rendering, sym := locatorModel(t, "action.sysml", "", "FlowViews::driveView", "Flows::Drive")
	graph, err := lower.ToActionGraph(sym.Decl, declScope(sym))
	if err != nil {
		t.Fatalf("ToActionGraph: %v", err)
	}
	loc, err := LocateActions(rendering, sym, sym, graph)
	if err != nil {
		t.Fatalf("LocateActions: %v", err)
	}
	agree := func(within []lower.FlowNode, decls []ast.Node, flow *lower.ActionGraph) (nodes, edges int) {
		t.Helper()
		for _, id := range flow.NodeIDs() {
			decl := flow.Provenance.Node(id)
			want, wantOK := loc.Node(decls, decl)
			got, ok := loc.NodeAt(within, lower.FlowNode{Graph: flow, ID: id})
			if got != want || ok != wantOK {
				t.Errorf("NodeAt(%s) = %q, %v; Node = %q, %v", behaviorNodeName(decl), got, ok, want, wantOK)
			}
			if ok {
				nodes++
			}
			for _, edge := range flow.EdgesOf(id) {
				want, wantOK := loc.Edge(decls, edge)
				got, ok := loc.EdgeAt(within, flow, edge)
				if got != want || ok != wantOK {
					t.Errorf("EdgeAt(from %s) = %d, %v; Edge = %d, %v", behaviorNodeName(decl), got, ok, want, wantOK)
				}
				if ok {
					edges++
				}
			}
		}
		return nodes, edges
	}
	if nodes, edges := agree(nil, nil, graph); nodes != 9 || edges != 11 {
		t.Errorf("located %d nodes and %d successions of Drive, want 9 and 11", nodes, edges)
	}

	var monitor lower.FlowNode
	for _, id := range graph.NodeIDs() {
		if behaviorNodeName(graph.Provenance.Node(id)) == "monitor" {
			monitor = lower.FlowNode{Graph: graph, ID: id}
		}
	}
	sub := graph.SubflowOf(monitor.ID)
	if sub == nil || sub.Graph == nil {
		t.Fatal("monitor states no flow of its own")
	}
	nested := sub.Graph
	decls := []ast.Node{graph.Provenance.Node(monitor.ID)}
	if nodes, edges := agree([]lower.FlowNode{monitor}, decls, nested); nodes != 2 || edges != 1 {
		t.Errorf("located %d nodes and %d successions of monitor, want 2 and 1", nodes, edges)
	}

	// No node, and a nesting the rendering does not draw, fall back to the
	// innermost drawn node, reported as not located.
	if id, ok := loc.NodeAt(nil, lower.FlowNode{}); ok || id != loc.Root() {
		t.Errorf("NodeAt(no node) = %q, %v; want the root, not located", id, ok)
	}
	record := lower.FlowNode{Graph: nested, ID: nested.NodeIDs()[len(nested.NodeIDs())-1]}
	id, ok := loc.NodeAt([]lower.FlowNode{monitor, record}, lower.FlowNode{})
	if ok || nodeName(t, rendering, id) != "record" {
		t.Errorf("NodeAt(monitor/record/...) = %q, %v; want record, not located", id, ok)
	}
}

// A succession a succession flow gates is located at the edge drawing the flow
// it carries, by id as by declaration.
func TestLocateActionsByIDFindsAGatedSuccession(t *testing.T) {
	rendering, sym := locatorModel(t, "gated-flow.sysml", "", "GatedViews::passView", "Gated::Pass")
	graph, err := lower.ToActionGraph(sym.Decl, declScope(sym))
	if err != nil {
		t.Fatalf("ToActionGraph: %v", err)
	}
	loc, err := LocateActions(rendering, sym, sym, graph)
	if err != nil {
		t.Fatalf("LocateActions: %v", err)
	}
	gated := 0
	for _, id := range graph.NodeIDs() {
		for _, edge := range graph.EdgesOf(id) {
			if !edge.Gated() {
				continue
			}
			gated++
			want, wantOK := loc.Edge(nil, edge)
			index, ok := loc.EdgeAt(nil, graph, edge)
			if index != want || ok != wantOK || !ok {
				t.Fatalf("EdgeAt(gated) = %d, %v; Edge = %d, %v", index, ok, want, wantOK)
			}
			if drawn := rendering.Data().Edges[index]; drawn.Kind != EdgeSuccessionFlow {
				t.Errorf("the gated succession is located at a %s edge, want the succession flow", drawn.Kind)
			}
			if got := edgeEnds(t, rendering, index); got != [2]string{"producer", "consumer"} {
				t.Errorf("the gated succession draws %v", got)
			}
		}
	}
	if gated != 1 {
		t.Errorf("the flow has %d gated successions, want 1", gated)
	}
}

// A parallel state's regions are owned in the graph by states the rendering
// does not draw: those answer for no node, and a fork written in a region is
// still located under the region rather than under its undrawn owner.
func TestLocateStatesSkipsGraphOnlyRegionOwners(t *testing.T) {
	const src = `package P {
	state Machine {
		entry; then idle;
		state idle;
		state work parallel {
			state left {
				fork split;
				state inner parallel {
					state upper {
						state a;
					}
					state lower {
						state b;
					}
				}
				transition first split then inner.upper.a;
				transition first split then inner.lower.b;
			}
			state right;
		}
		transition first idle then work.left.split;
	}
}`
	r, idx := loadSources(t, []string{"owners.sysml"}, [][]byte{[]byte(src)})
	sym := lookup(t, idx, "P::Machine")
	rendering, err := r.RenderExposed([]*symbols.Symbol{sym}, KindState, "")
	if err != nil {
		t.Fatalf("RenderExposed: %v", err)
	}
	graph, err := lower.ToStateGraph(sym.Decl, declScope(sym))
	if err != nil {
		t.Fatalf("ToStateGraph: %v", err)
	}
	loc, err := LocateStates(rendering, sym, sym, graph)
	if err != nil {
		t.Fatalf("LocateStates: %v", err)
	}
	forks := 0
	for _, id := range graph.PseudoIDs() {
		node, ok := loc.Vertex(program.PseudoVertex(id))
		if !ok {
			t.Fatalf("pseudostate %d is not located", id)
		}
		if owner := graph.PseudostateOwnerOf(id); owner != 0 {
			if got, ok := loc.Vertex(program.StateVertex(owner)); ok {
				t.Errorf("the graph-only owner of pseudostate %d is located at %q", id, got)
			}
			if got, ok := loc.StateStart(owner); ok {
				t.Errorf("the graph-only owner of pseudostate %d has a start marker %q", id, got)
			}
		}
		if nodeName(t, rendering, node) == "split" {
			forks++
		}
	}
	if forks != 1 {
		t.Errorf("located %d forks named split, want 1", forks)
	}
	drawn := make(map[program.StateID]bool)
	for _, id := range graph.StateIDs() {
		drawn[id] = true
		for _, tr := range graph.TransitionsOf(program.StateVertex(id)) {
			if _, ok := loc.TransitionEdge(tr); !ok {
				t.Errorf("a transition out of %s is not located", graph.Provenance.State(id).Name)
			}
		}
	}
	owners := 0
	for _, id := range graph.StateIDs() {
		for _, region := range graph.RegionsOf(id) {
			if _, ok := loc.Region(region); !ok {
				t.Errorf("region %d is not located", region)
			}
			if owner := graph.RegionStateOf(region); owner != 0 && !drawn[owner] {
				owners++
				if got, ok := loc.Vertex(program.StateVertex(owner)); ok {
					t.Errorf("graph-only region state %d is located at %q", owner, got)
				}
			}
		}
	}
	if owners != 4 {
		t.Errorf("found %d graph-only region states, want 4", owners)
	}
}
