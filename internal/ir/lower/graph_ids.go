package lower

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// The records and tables of a lowered graph name their nodes and vertices by declaration;
// what follows names the same ones by the ids of the graph's identity tables.

// FlowNode names a node of a flow by its id there; the zero FlowNode names none.
type FlowNode struct {
	Graph *ActionGraph
	ID    program.NodeID
}

// stampIDs writes, beside each node a record names, the id the graph gave it.
func (g *ActionGraph) stampIDs() {
	for _, edges := range g.Edges {
		for i := range edges {
			edges[i].SourceID, edges[i].TargetID = g.ID[edges[i].Source], g.ID[edges[i].Target]
		}
	}
	for source, flows := range g.DataFlows {
		for i := range flows {
			flows[i].SourceID, flows[i].TargetID = g.ID[source], g.ID[flows[i].Target]
		}
	}
	for _, bindings := range [][]PinBinding{g.Bindings, g.ValueBindings} {
		for i := range bindings {
			bindings[i] = g.withBindingIDs(bindings[i])
		}
	}
}

// withBindingIDs returns binding with the ids of the nodes its two ends name.
func (g *ActionGraph) withBindingIDs(binding PinBinding) PinBinding {
	binding.NodeID, binding.OtherNodeID = g.ID[binding.Node], g.ID[binding.OtherNode]
	return binding
}

// nodeIDs maps nodes of the graph to their ids, in order.
func (g *ActionGraph) nodeIDs(nodes []ast.Node) []program.NodeID {
	if len(nodes) == 0 {
		return nil
	}
	ids := make([]program.NodeID, len(nodes))
	for i, node := range nodes {
		ids[i] = g.ID[node]
	}
	return ids
}

// NodeIDs are the ids of the graph's nodes, in graph order.
func (g *ActionGraph) NodeIDs() []program.NodeID { return g.nodeIDs(g.Nodes) }

// InitialID is the node the flow's ordered part starts at, 0 for none.
func (g *ActionGraph) InitialID() program.NodeID { return g.ID[g.Initial] }

// ConcurrentIDs are the composite subactions no succession leads to, in declaration order.
func (g *ActionGraph) ConcurrentIDs() []program.NodeID { return g.nodeIDs(g.Concurrent) }

// StartIDs are the nodes a performance of the flow starts at: Starts by id.
func (g *ActionGraph) StartIDs() []program.NodeID { return g.nodeIDs(g.Starts()) }

// FinalIDs are the flow's final nodes.
func (g *ActionGraph) FinalIDs() []program.NodeID { return g.nodeIDs(g.Finals) }

// EnclosingNodeID is the node of Enclosing the flow runs under, 0 for the outermost flow.
func (g *ActionGraph) EnclosingNodeID() program.NodeID {
	if g.Enclosing == nil {
		return 0
	}
	return g.Enclosing.ID[g.EnclosingNode]
}

// EdgesOf are the successions out of the node, in declaration order; shared, not to be modified.
func (g *ActionGraph) EdgesOf(id program.NodeID) []ActionEdge {
	return g.Edges[g.Provenance.Node(id)]
}

// IncomingOf are the successions into the node: Incoming by id.
func (g *ActionGraph) IncomingOf(id program.NodeID) []ActionEdge {
	return g.Incoming(g.Provenance.Node(id))
}

// FlowsOf are the object flows out of the node; shared, not to be modified.
func (g *ActionGraph) FlowsOf(id program.NodeID) []ObjectFlow {
	return g.DataFlows[g.Provenance.Node(id)]
}

// BodyOf are the statements the node executes, in declaration order.
func (g *ActionGraph) BodyOf(id program.NodeID) []Statement {
	return g.Bodies[g.Provenance.Node(id)]
}

// FeaturesOf are the parameters and attributes the node declares itself, in declaration order.
func (g *ActionGraph) FeaturesOf(id program.NodeID) []Feature {
	return g.Features[g.Provenance.Node(id)]
}

// AcceptOf is the message the node waits for, false for a node that accepts none.
func (g *ActionGraph) AcceptOf(id program.NodeID) (Accept, bool) {
	accept, ok := g.Accepts[g.Provenance.Node(id)]
	return accept, ok
}

// SubflowOf is the flow the node's own members state, nil for a node stating none.
func (g *ActionGraph) SubflowOf(id program.NodeID) *Subflow {
	return g.Subflows[g.Provenance.Node(id)]
}

// StatementRun reports whether the node stands for a run of statements of a block's own flow.
func (g *ActionGraph) StatementRun(id program.NodeID) bool {
	return g.StatementRuns[g.Provenance.Node(id)]
}

// BlockNodesOf are the action nodes the blocks of the node's body declare, in
// declaration order, each in the block flow it is a node of.
func (g *ActionGraph) BlockNodesOf(id program.NodeID) []FlowNode {
	owner := g.Provenance.Node(id)
	nodes := g.BlockNodes[owner]
	if len(nodes) == 0 {
		return nil
	}
	refs := make([]FlowNode, 0, len(nodes))
	for _, node := range nodes {
		if flow := NestedFlow(g, owner, node); flow != nil {
			refs = append(refs, FlowNode{Graph: flow, ID: flow.ID[node]})
		}
	}
	return refs
}

// MachineID is the id of the machine state, which the state table leaves out: one
// past the last numbered state; 0 for nil and for a graph whose states are not numbered.
func (g *StateGraph) MachineID() program.StateID {
	if g == nil {
		return 0
	}
	return g.machineID
}

// StateIDOf is the id of a state of the graph, the machine state included; 0 for nil.
func (g *StateGraph) StateIDOf(state *ast.StateNode) program.StateID {
	if state == nil {
		return 0
	}
	if state == g.Machine {
		return g.MachineID()
	}
	return g.ID[state]
}

// stateOf is the state under id, the machine state included; nil for 0.
func (g *StateGraph) stateOf(id program.StateID) *ast.StateNode {
	if g.Machine != nil && id == g.MachineID() {
		return g.Machine
	}
	return g.Provenance.State(id)
}

// VertexOf names a state, pseudostate or terminate action of the graph by id; the
// zero reference for nil or any other node.
func (g *StateGraph) VertexOf(node ast.Node) program.VertexRef {
	switch n := node.(type) {
	case *ast.StateNode:
		return program.StateVertex(g.StateIDOf(n))
	case *ast.PseudostateNode:
		if n != nil {
			return program.PseudoVertex(g.PseudoID[n])
		}
	case *ast.Usage:
		if i := slices.Index(g.Terminates, n); i >= 0 {
			return program.TerminateVertex(tableIndex(i + 1))
		}
	}
	return program.VertexRef{}
}

// vertexDecl is the declaration ref names, nil for the zero reference.
func (g *StateGraph) vertexDecl(ref program.VertexRef) ast.Node {
	switch ref.Kind {
	case program.VertexState:
		if state := g.stateOf(ref.State()); state != nil {
			return state
		}
	case program.VertexPseudostate:
		if ps := g.Provenance.Pseudostate(ref.Pseudo()); ps != nil {
			return ps
		}
	case program.VertexTerminate:
		if i := int(ref.Index) - 1; i >= 0 && i < len(g.Terminates) {
			return g.Terminates[i]
		}
	}
	return nil
}

// stampIDs writes, beside each state and vertex a record names, the id the graph gave it.
func (g *StateGraph) stampIDs() {
	behaviors := func(list []StateBehavior) {
		for i := range list {
			list[i].OwnerID = g.StateIDOf(list[i].Owner)
		}
	}
	for _, own := range g.Behaviors {
		behaviors(own.Entry)
		behaviors(own.Do)
		behaviors(own.Exit)
	}
	for usage, behavior := range g.TerminateBehaviors {
		behavior.OwnerID = g.StateIDOf(behavior.Owner)
		g.TerminateBehaviors[usage] = behavior
	}
	for _, transitions := range g.Transitions {
		for _, trans := range transitions {
			trans.SourceRef, trans.TargetRef = g.VertexOf(trans.Source), g.VertexOf(trans.Target)
			trans.OwnerID = g.StateIDOf(trans.Owner)
			behaviors(trans.Effect)
		}
	}
	for _, entries := range g.EntryTransitions {
		for _, entry := range entries {
			entry.TargetID = g.StateIDOf(entry.Target)
			if entry.Via != nil {
				entry.ViaID = g.PseudoID[entry.Via]
			}
			behaviors(entry.Effect)
		}
	}
}

// stateIDs maps states of the graph to their ids, in order.
func (g *StateGraph) stateIDs(states []*ast.StateNode) []program.StateID {
	if len(states) == 0 {
		return nil
	}
	ids := make([]program.StateID, len(states))
	for i, state := range states {
		ids[i] = g.StateIDOf(state)
	}
	return ids
}

// regionIDs maps regions of the graph to their ids, in order.
func (g *StateGraph) regionIDs(regions []*ast.StateRegion) []program.RegionID {
	if len(regions) == 0 {
		return nil
	}
	ids := make([]program.RegionID, len(regions))
	for i, region := range regions {
		ids[i] = g.RegionID[region]
	}
	return ids
}

// StateIDs are the machine's states, nested ones included, in the order of States.
func (g *StateGraph) StateIDs() []program.StateID { return g.stateIDs(g.States) }

// InitialID is the state a machine without top-level regions starts in, 0 for none.
func (g *StateGraph) InitialID() program.StateID { return g.StateIDOf(g.Initial) }

// ParentOf is the state's parent, 0 for a state without one.
func (g *StateGraph) ParentOf(id program.StateID) program.StateID {
	return g.StateIDOf(g.ParentState[g.stateOf(id)])
}

// StateRegion is the region that declares the state, 0 for a state in none.
func (g *StateGraph) StateRegion(id program.StateID) program.RegionID {
	return g.RegionID[g.RegionOf[g.stateOf(id)]]
}

// RegionsOf are the composite state's regions, in declaration order.
func (g *StateGraph) RegionsOf(id program.StateID) []program.RegionID {
	return g.regionIDs(g.CompositeStates[g.stateOf(id)])
}

// TopRegionIDs are the machine's own orthogonal regions, in declaration order.
func (g *StateGraph) TopRegionIDs() []program.RegionID { return g.regionIDs(g.TopRegions) }

// RegionOwnerOf is the composite state owning the region, 0 for a region of the machine.
func (g *StateGraph) RegionOwnerOf(id program.RegionID) program.StateID {
	return g.StateIDOf(g.RegionOwner[g.Provenance.Region(id)])
}

// RegionInitialOf is the state the region starts in, 0 when default entry leaves it inactive.
func (g *StateGraph) RegionInitialOf(id program.RegionID) program.StateID {
	return g.StateIDOf(g.RegionInitials[g.Provenance.Region(id)])
}

// RegionStateOf is the graph-only owner standing for a synthesized region, 0 for none.
func (g *StateGraph) RegionStateOf(id program.RegionID) program.StateID {
	return g.StateIDOf(g.RegionState[g.Provenance.Region(id)])
}

// HiddenRegion is the region a graph-only owner stands for, 0 for a visible state.
func (g *StateGraph) HiddenRegion(id program.StateID) program.RegionID {
	return g.RegionID[g.HiddenRegionOf[g.stateOf(id)]]
}

// Hidden reports whether the state is a graph-only owner synthesized for a parallel region.
func (g *StateGraph) Hidden(id program.StateID) bool { return g.HiddenStates[g.stateOf(id)] }

// ParallelState reports whether the state's substates run in parallel.
func (g *StateGraph) ParallelState(id program.StateID) bool {
	if g.Machine != nil && id == g.MachineID() {
		return g.MachineParallel
	}
	return g.Parallel[g.stateOf(id)]
}

// BehaviorsOf are the state's lowered entry, do and exit behaviors, nil for a state with none.
func (g *StateGraph) BehaviorsOf(id program.StateID) *StateBehaviors {
	return g.Behaviors[g.stateOf(id)]
}

// AttributesOf are the attributes the state owns, its own and those it inherits.
func (g *StateGraph) AttributesOf(id program.StateID) []Attribute {
	return g.StateAttributes[g.stateOf(id)]
}

// BehaviorFeatures are the directed parameters an entry, do or exit action usage declares.
func (g *StateGraph) BehaviorFeatures(behavior StateBehavior) []Feature {
	return g.Features[behavior.Node]
}

// TransitionsOf are the transitions leaving the vertex, in declaration order.
func (g *StateGraph) TransitionsOf(ref program.VertexRef) []*Transition {
	source := g.vertexDecl(ref)
	if source == nil {
		return nil
	}
	return g.Transitions[source]
}

// EntryTransitionsOf are the transitions out of the entry action of the state's body,
// in declaration order; 0 is the machine's own body.
func (g *StateGraph) EntryTransitionsOf(id program.StateID) []*EntryTransition {
	if id == 0 {
		return g.EntryTransitions[nil]
	}
	state := g.stateOf(id)
	if state == nil {
		return nil
	}
	return g.EntryTransitions[state]
}

// RegionEntryTransitions are the transitions out of the entry action of the region's body.
func (g *StateGraph) RegionEntryTransitions(id program.RegionID) []*EntryTransition {
	region := g.Provenance.Region(id)
	if region == nil {
		return nil
	}
	return g.EntryTransitions[region]
}

// PseudostateOwnerOf is the composite state declaring the pseudostate, 0 for one of the machine.
func (g *StateGraph) PseudostateOwnerOf(id program.PseudoID) program.StateID {
	return g.StateIDOf(g.PseudostateOwner[g.Provenance.Pseudostate(id)])
}

// PseudostateRegionOf is the region the pseudostate's outgoing segments run within, 0 for none.
func (g *StateGraph) PseudostateRegionOf(id program.PseudoID) program.RegionID {
	return g.RegionID[g.PseudostateRegion[g.Provenance.Pseudostate(id)]]
}

// TerminateOwnerOf is the composite state declaring the terminate action ref names,
// 0 for one of the machine or another kind of vertex.
func (g *StateGraph) TerminateOwnerOf(ref program.VertexRef) program.StateID {
	stop, _ := g.vertexDecl(ref).(*ast.Usage)
	if stop == nil {
		return 0
	}
	return g.StateIDOf(g.TerminateOwner[stop])
}

// PseudoIDs are the ids of the graph's pseudostates, in declaration order.
func (g *StateGraph) PseudoIDs() []program.PseudoID {
	if len(g.Pseudostates) == 0 {
		return nil
	}
	ids := make([]program.PseudoID, len(g.Pseudostates))
	for i, ps := range g.Pseudostates {
		ids[i] = g.PseudoID[ps]
	}
	return ids
}

// CompositeIDs are the ids of the states owning regions, in declaration order.
func (g *StateGraph) CompositeIDs() []program.StateID { return g.stateIDs(g.CompositeStateOrder) }

// StateScope is the namespace the state owns, nil for none.
func (g *StateGraph) StateScope(id program.StateID) *symbols.Scope {
	return g.StateScopes[g.stateOf(id)]
}
