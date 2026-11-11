package runtime

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// The executor's own records name the graph's states, regions, pseudostates and
// transitions by the graph-local ids lowering assigns; these translate between
// an id and the declaration the graph's tables are still keyed by.

// machineID is the id the executor gives the machine state, which lowering keeps
// out of the state table: one past the last numbered state.
func (e *StateExecutor) machineID() program.StateID {
	return program.StateID(len(e.graph.Origins)) // #nosec G115 -- a graph has far fewer than 2^32 states
}

// stateID is the graph-local id of state, 0 for nil.
func (e *StateExecutor) stateID(state *ast.StateNode) program.StateID {
	if state == nil {
		return 0
	}
	if state == e.graph.Machine {
		return e.machineID()
	}
	return e.graph.ID[state]
}

// stateDecl is the state declared under id, nil for 0.
func (e *StateExecutor) stateDecl(id program.StateID) *ast.StateNode {
	if id == e.machineID() && e.graph.Machine != nil {
		return e.graph.Machine
	}
	return e.graph.Provenance.State(id)
}

// stateIDs maps states to their ids, in order.
func (e *StateExecutor) stateIDs(states []*ast.StateNode) []program.StateID {
	if states == nil {
		return nil
	}
	ids := make([]program.StateID, len(states))
	for i, state := range states {
		ids[i] = e.stateID(state)
	}
	return ids
}

// stateDecls maps ids to the states declared under them, in order.
func (e *StateExecutor) stateDecls(ids []program.StateID) []*ast.StateNode {
	if ids == nil {
		return nil
	}
	states := make([]*ast.StateNode, len(ids))
	for i, id := range ids {
		states[i] = e.stateDecl(id)
	}
	return states
}

// regionID is the graph-local id of region, 0 for nil.
func (e *StateExecutor) regionID(region *ast.StateRegion) program.RegionID {
	if region == nil {
		return 0
	}
	return e.graph.RegionID[region]
}

// regionDecl is the region declared under id, nil for 0.
func (e *StateExecutor) regionDecl(id program.RegionID) *ast.StateRegion {
	return e.graph.Provenance.Region(id)
}

// regionIDs maps regions to their ids, in order.
func (e *StateExecutor) regionIDs(regions []*ast.StateRegion) []program.RegionID {
	if regions == nil {
		return nil
	}
	ids := make([]program.RegionID, len(regions))
	for i, region := range regions {
		ids[i] = e.regionID(region)
	}
	return ids
}

// regionDecls maps ids to the regions declared under them, in order.
func (e *StateExecutor) regionDecls(ids []program.RegionID) []*ast.StateRegion {
	if ids == nil {
		return nil
	}
	regions := make([]*ast.StateRegion, len(ids))
	for i, id := range ids {
		regions[i] = e.regionDecl(id)
	}
	return regions
}

// pseudoID is the graph-local id of ps, 0 for nil.
func (e *StateExecutor) pseudoID(ps *ast.PseudostateNode) program.PseudoID {
	if ps == nil {
		return 0
	}
	return e.graph.PseudoID[ps]
}

// pseudoDecl is the pseudostate declared under id, nil for 0.
func (e *StateExecutor) pseudoDecl(id program.PseudoID) *ast.PseudostateNode {
	return e.graph.Provenance.Pseudostate(id)
}

// transID is the graph-local id of trans, 0 for nil.
func (e *StateExecutor) transID(trans *lower.Transition) program.TransID {
	if trans == nil {
		return 0
	}
	return e.graph.TransID[trans]
}

// transDecl is the transition lowered under id, nil for 0.
func (e *StateExecutor) transDecl(id program.TransID) *lower.Transition {
	return e.graph.Provenance.Transition(id)
}

// transIDs maps transitions to their ids, in order.
func (e *StateExecutor) transIDs(transitions []*lower.Transition) []program.TransID {
	if transitions == nil {
		return nil
	}
	ids := make([]program.TransID, len(transitions))
	for i, trans := range transitions {
		ids[i] = e.transID(trans)
	}
	return ids
}

// vertexRef names a state, pseudostate or terminate action vertex by id; the zero
// reference for nil or any other node.
func (e *StateExecutor) vertexRef(node ast.Node) program.VertexRef {
	switch n := node.(type) {
	case *ast.StateNode:
		return program.StateVertex(e.stateID(n))
	case *ast.PseudostateNode:
		return program.PseudoVertex(e.pseudoID(n))
	case *ast.Usage:
		if i := slices.Index(e.graph.Terminates, n); i >= 0 {
			return program.TerminateVertex(uint32(i + 1)) // #nosec G115 -- a graph has far fewer than 2^32 terminates
		}
	}
	return program.VertexRef{}
}

// vertexDecl is the node declared under ref, nil for the zero reference.
func (e *StateExecutor) vertexDecl(ref program.VertexRef) ast.Node {
	switch ref.Kind {
	case program.VertexState:
		if state := e.stateDecl(ref.State()); state != nil {
			return state
		}
	case program.VertexPseudostate:
		if ps := e.pseudoDecl(ref.Pseudo()); ps != nil {
			return ps
		}
	case program.VertexTerminate:
		if i := int(ref.Index) - 1; i >= 0 && i < len(e.graph.Terminates) {
			return e.graph.Terminates[i]
		}
	}
	return nil
}

// terminateDecl is the terminate action usage under ref, nil for any other reference.
func (e *StateExecutor) terminateDecl(ref program.VertexRef) *ast.Usage {
	stop, _ := e.vertexDecl(ref).(*ast.Usage)
	return stop
}

// regionStateIDs maps a region → state table to ids.
func (e *StateExecutor) regionStateIDs(states map[*ast.StateRegion]*ast.StateNode) map[program.RegionID]program.StateID {
	if states == nil {
		return nil
	}
	ids := make(map[program.RegionID]program.StateID, len(states))
	for region, state := range states {
		ids[e.regionID(region)] = e.stateID(state)
	}
	return ids
}

// regionStateDecls maps a region → state table of ids back to declarations.
func (e *StateExecutor) regionStateDecls(ids map[program.RegionID]program.StateID) map[*ast.StateRegion]*ast.StateNode {
	if ids == nil {
		return nil
	}
	states := make(map[*ast.StateRegion]*ast.StateNode, len(ids))
	for region, state := range ids {
		states[e.regionDecl(region)] = e.stateDecl(state)
	}
	return states
}
