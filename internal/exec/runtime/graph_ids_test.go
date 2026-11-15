package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/tests/fixtures"
)

// TestLoweredGraphIDs lowers every action and state machine of the conformance models and
// checks that each node, state and vertex a record names by declaration it names by id too.
func TestLoweredGraphIDs(t *testing.T) {
	dir := filepath.Join("testdata", "conformance")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	actions, machines := 0, 0
	for _, entry := range entries {
		if !isConformanceCase(entry.Name()) {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".expected.json")
		path := fixtures.ModelPath(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var expected ExpectedOutcome
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &expected); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		src := source.New(path, data)
		file := parser.New(src).ParseFile()
		idx, _ := indexCaseDocuments(t, dir, src, file, expected)
		resolver := resolve.New(idx)
		check := graphIDCheck{t: t, model: name, seen: map[*lower.ActionGraph]bool{}}
		var walk func(scope *symbols.Scope)
		walk = func(scope *symbols.Scope) {
			if scope == nil {
				return
			}
			for _, sym := range scope.Members() {
				switch {
				case sym.Decl == nil:
				case sym.Kind == symbols.SymbolStateUsage || sym.Kind == symbols.SymbolStateDef:
					graph, err := lower.ToStateGraphWithEndpoints(sym.Decl, DeclScope(sym), lower.NewLibraryStateTypes(resolver))
					if err == nil {
						machines++
						check.state(graph)
					}
				case sym.Kind.IsAction():
					graph, err := lower.ToActionGraphWith(sym.Decl, DeclScope(sym), resolver)
					if err == nil {
						actions++
						lower.StartFlow(graph)
						check.action(graph)
					}
				}
			}
			for _, child := range scope.Children() {
				walk(child)
			}
		}
		walk(idx.DocumentRoot(src.Name()))
	}
	if actions == 0 || machines == 0 {
		t.Fatalf("lowered %d actions and %d state machines, want some of each", actions, machines)
	}
}

type graphIDCheck struct {
	t     *testing.T
	model string
	seen  map[*lower.ActionGraph]bool
}

func (c *graphIDCheck) errorf(format string, args ...any) {
	c.t.Helper()
	c.t.Errorf(c.model+": "+format, args...)
}

// node checks that id is node's in g: non-zero and back to node through the provenance.
func (c *graphIDCheck) node(g *lower.ActionGraph, what string, node ast.Node, id program.NodeID) {
	c.t.Helper()
	if node == nil {
		if id != 0 {
			c.errorf("%s: id %d for no node", what, id)
		}
		return
	}
	if id == 0 || g.Provenance.Node(id) != node {
		c.errorf("%s: id %d does not name %T at %v", what, id, node, node.Span())
	}
}

func (c *graphIDCheck) nodes(g *lower.ActionGraph, what string, nodes []ast.Node, ids []program.NodeID) {
	c.t.Helper()
	if len(nodes) != len(ids) {
		c.errorf("%s: %d ids for %d nodes", what, len(ids), len(nodes))
		return
	}
	for i, node := range nodes {
		c.node(g, what, node, ids[i])
	}
}

func (c *graphIDCheck) statements(stmts []lower.Statement) {
	for _, flow := range lower.BlockFlows(stmts) {
		c.action(flow)
	}
}

func (c *graphIDCheck) action(g *lower.ActionGraph) {
	if g == nil || c.seen[g] {
		return
	}
	c.seen[g] = true
	if len(g.Origins) != len(g.Kinds) {
		c.errorf("%d origins for %d kinds", len(g.Origins), len(g.Kinds))
	}
	c.node(g, "Initial", g.Initial, g.InitialID())
	c.nodes(g, "Concurrent", g.Concurrent, g.ConcurrentIDs())
	c.nodes(g, "Starts", g.Starts(), g.StartIDs())
	c.nodes(g, "Finals", g.Finals, g.FinalIDs())
	if g.Enclosing != nil {
		c.node(g.Enclosing, "EnclosingNode", g.EnclosingNode, g.EnclosingNodeID())
	}
	keyed := map[ast.Node]bool{}
	for node := range g.Edges {
		keyed[node] = true
	}
	for node := range g.DataFlows {
		keyed[node] = true
	}
	for node := range g.Bodies {
		keyed[node] = true
	}
	for node := range g.Features {
		keyed[node] = true
	}
	for node := range g.Accepts {
		keyed[node] = true
	}
	for node := range g.Subflows {
		keyed[node] = true
	}
	for node := range g.StatementRuns {
		keyed[node] = true
	}
	for node := range g.BlockNodes {
		keyed[node] = true
	}
	for node := range keyed {
		c.node(g, "table key", node, g.ID[node])
	}
	for i := 1; i < len(g.Origins); i++ {
		id := program.NodeID(i) // #nosec G115 -- a table index
		node := g.Provenance.Node(id)
		c.node(g, "ID", node, g.ID[node])
		edges := g.EdgesOf(id)
		if len(edges) != len(g.Edges[node]) {
			c.errorf("EdgesOf(%d): %d edges, want %d", id, len(edges), len(g.Edges[node]))
		}
		for _, edge := range edges {
			c.node(g, "ActionEdge.Source", edge.Source, edge.SourceID)
			c.node(g, "ActionEdge.Target", edge.Target, edge.TargetID)
			if edge.SourceID != id || edge.TargetID == 0 {
				c.errorf("edge %d -> %d listed under %d", edge.SourceID, edge.TargetID, id)
			}
		}
		for _, edge := range g.IncomingOf(id) {
			if edge.TargetID != id || edge.SourceID == 0 {
				c.errorf("edge %d -> %d listed as incoming to %d", edge.SourceID, edge.TargetID, id)
			}
		}
		flows := g.FlowsOf(id)
		if len(flows) != len(g.DataFlows[node]) {
			c.errorf("FlowsOf(%d): %d flows, want %d", id, len(flows), len(g.DataFlows[node]))
		}
		for _, flow := range flows {
			c.node(g, "ObjectFlow.Target", flow.Target, flow.TargetID)
			if flow.SourceID != id {
				c.errorf("flow from %d listed under %d", flow.SourceID, id)
			}
		}
		if len(g.BodyOf(id)) != len(g.Bodies[node]) || len(g.FeaturesOf(id)) != len(g.Features[node]) {
			c.errorf("BodyOf/FeaturesOf(%d) differ from the node's", id)
		}
		if _, ok := g.AcceptOf(id); ok != (g.Kinds[id] == program.NodeAccept && ok) {
			c.errorf("AcceptOf(%d) on a %s node", id, g.Kinds[id])
		}
		if g.SubflowOf(id) != g.Subflows[node] || g.StatementRun(id) != g.StatementRuns[node] {
			c.errorf("SubflowOf/StatementRun(%d) differ from the node's", id)
		}
		block := g.BlockNodesOf(id)
		if len(block) != len(g.BlockNodes[node]) {
			c.errorf("BlockNodesOf(%d): %d nodes, want %d", id, len(block), len(g.BlockNodes[node]))
		} else {
			for i, ref := range block {
				c.node(ref.Graph, "BlockNodes", g.BlockNodes[node][i], ref.ID)
			}
		}
		c.statements(g.Bodies[node])
		if sub := g.Subflows[node]; sub != nil {
			c.action(sub.Graph)
		}
	}
	for _, bindings := range [][]lower.PinBinding{g.Bindings, g.ValueBindings} {
		for _, b := range bindings {
			c.node(g, "PinBinding.Node", b.Node, b.NodeID)
			c.node(g, "PinBinding.OtherNode", b.OtherNode, b.OtherNodeID)
		}
	}
}

// stateID checks that id is state's in g, 0 exactly for nil.
func (c *graphIDCheck) stateID(g *lower.StateGraph, what string, state *ast.StateNode, id program.StateID) {
	c.t.Helper()
	if id != g.StateIDOf(state) || (state != nil) != (id != 0) {
		c.errorf("%s: id %d, want %d", what, id, g.StateIDOf(state))
	}
}

func (c *graphIDCheck) regionID(g *lower.StateGraph, what string, region *ast.StateRegion, id program.RegionID) {
	c.t.Helper()
	if (region != nil) != (id != 0) || g.Provenance.Region(id) != region {
		c.errorf("%s: id %d does not name the region", what, id)
	}
}

func (c *graphIDCheck) vertex(g *lower.StateGraph, what string, node ast.Node, ref program.VertexRef) {
	c.t.Helper()
	if ref != g.VertexOf(node) || (node != nil) != !ref.IsZero() {
		c.errorf("%s: %v, want %v", what, ref, g.VertexOf(node))
	}
}

func (c *graphIDCheck) behaviors(g *lower.StateGraph, what string, list []lower.StateBehavior) {
	c.t.Helper()
	for _, behavior := range list {
		c.stateID(g, what+" behavior owner", behavior.Owner, behavior.OwnerID)
		if len(g.BehaviorFeatures(behavior)) != len(g.Features[behavior.Node]) {
			c.errorf("%s: BehaviorFeatures differ from the behavior's", what)
		}
		c.statements(behavior.Body)
	}
}

func (c *graphIDCheck) state(g *lower.StateGraph) {
	for i := 1; i < len(g.Origins); i++ {
		id := program.StateID(i) // #nosec G115 -- a table index
		state := g.Provenance.State(id)
		c.stateID(g, "ID", state, id)
		c.stateID(g, "ParentState", g.ParentState[state], g.ParentOf(id))
		c.regionID(g, "RegionOf", g.RegionOf[state], g.StateRegion(id))
		c.regionID(g, "HiddenRegionOf", g.HiddenRegionOf[state], g.HiddenRegion(id))
		regions := g.RegionsOf(id)
		if len(regions) != len(g.CompositeStates[state]) {
			c.errorf("RegionsOf(%d): %d regions, want %d", id, len(regions), len(g.CompositeStates[state]))
			continue
		}
		for i, region := range g.CompositeStates[state] {
			c.regionID(g, "CompositeStates", region, regions[i])
		}
		if g.Hidden(id) != g.HiddenStates[state] || g.ParallelState(id) != g.Parallel[state] ||
			len(g.AttributesOf(id)) != len(g.StateAttributes[state]) {
			c.errorf("Hidden/ParallelState/AttributesOf(%d) differ from the state's", id)
		}
	}
	if ids := g.StateIDs(); len(ids) != len(g.States) || slices.Contains(ids, 0) {
		c.errorf("StateIDs: %v for %d states", ids, len(g.States))
	}
	c.stateID(g, "Initial", g.Initial, g.InitialID())
	if g.Machine != nil {
		c.stateID(g, "Machine", g.Machine, g.MachineID())
	}
	for state, own := range g.Behaviors {
		id := g.StateIDOf(state)
		if id == 0 || g.BehaviorsOf(id) != own {
			c.errorf("Behaviors: state id %d does not reach its behaviors", id)
		}
		c.behaviors(g, "entry", own.Entry)
		c.behaviors(g, "do", own.Do)
		c.behaviors(g, "exit", own.Exit)
	}
	for _, table := range []map[*ast.StateNode]*ast.StateNode{g.ParentState} {
		for child := range table {
			if g.StateIDOf(child) == 0 {
				c.errorf("ParentState: a child without an id")
			}
		}
	}
	for state := range g.RegionOf {
		if g.StateIDOf(state) == 0 {
			c.errorf("RegionOf: a state without an id")
		}
	}
	tops := g.TopRegionIDs()
	if len(tops) != len(g.TopRegions) {
		c.errorf("TopRegionIDs: %d ids for %d regions", len(tops), len(g.TopRegions))
	}
	for i := 1; i < len(g.RegionOrigins); i++ {
		id := program.RegionID(i) // #nosec G115 -- a table index
		region := g.Provenance.Region(id)
		c.regionID(g, "RegionID", region, g.RegionID[region])
		c.stateID(g, "RegionOwner", g.RegionOwner[region], g.RegionOwnerOf(id))
		c.stateID(g, "RegionInitials", g.RegionInitials[region], g.RegionInitialOf(id))
		c.stateID(g, "RegionState", g.RegionState[region], g.RegionStateOf(id))
		c.entries(g, g.EntryTransitions[region], g.RegionEntryTransitions(id))
	}
	for i := 1; i < len(g.PseudoOrigins); i++ {
		id := program.PseudoID(i) // #nosec G115 -- a table index
		ps := g.Provenance.Pseudostate(id)
		c.vertex(g, "PseudoID", ps, program.PseudoVertex(id))
		c.stateID(g, "PseudostateOwner", g.PseudostateOwner[ps], g.PseudostateOwnerOf(id))
		c.regionID(g, "PseudostateRegion", g.PseudostateRegion[ps], g.PseudostateRegionOf(id))
	}
	for i, stop := range g.Terminates {
		ref := program.TerminateVertex(uint32(i + 1)) // #nosec G115 -- a table index
		c.vertex(g, "Terminates", stop, ref)
		c.stateID(g, "TerminateOwner", g.TerminateOwner[stop], g.TerminateOwnerOf(ref))
	}
	for _, behavior := range g.TerminateBehaviors {
		c.behaviors(g, "terminate", []lower.StateBehavior{behavior})
	}
	listed := 0
	for source, transitions := range g.Transitions {
		listed += len(transitions)
		ref := g.VertexOf(source)
		if ref.IsZero() || len(g.TransitionsOf(ref)) != len(transitions) {
			c.errorf("Transitions: source %v does not reach its %d transitions", ref, len(transitions))
		}
		for _, trans := range transitions {
			if g.TransID[trans] == 0 || trans.SourceRef != ref {
				c.errorf("transition %d from %v listed under %v", g.TransID[trans], trans.SourceRef, ref)
			}
			c.vertex(g, "Transition.Source", trans.Source, trans.SourceRef)
			c.vertex(g, "Transition.Target", trans.Target, trans.TargetRef)
			c.stateID(g, "Transition.Owner", trans.Owner, trans.OwnerID)
			c.behaviors(g, "effect", trans.Effect)
		}
	}
	if listed != len(g.TransOrigins)-1 {
		c.errorf("%d transitions listed, %d numbered", listed, len(g.TransOrigins)-1)
	}
	c.entries(g, g.EntryTransitions[nil], g.EntryTransitionsOf(0))
	for body, entries := range g.EntryTransitions {
		if state, ok := body.(*ast.StateNode); ok {
			c.entries(g, entries, g.EntryTransitionsOf(g.StateIDOf(state)))
		}
	}
}

func (c *graphIDCheck) entries(g *lower.StateGraph, want, got []*lower.EntryTransition) {
	c.t.Helper()
	if !slices.Equal(want, got) {
		c.errorf("entry transitions by id: %d, want %d", len(got), len(want))
	}
	for _, entry := range want {
		c.stateID(g, "EntryTransition.Target", entry.Target, entry.TargetID)
		if (entry.Via != nil) != (entry.ViaID != 0) || g.Provenance.Pseudostate(entry.ViaID) != entry.Via {
			c.errorf("EntryTransition.Via: id %d does not name it", entry.ViaID)
		}
		c.behaviors(g, "entry transition", entry.Effect)
	}
}
