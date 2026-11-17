package runtime

import (
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// NodeRef names one node of an action graph by the id the graph gives it: what the
// executor's records hold in place of the node, which Node reads back through the graph.
type NodeRef struct {
	Graph *lower.ActionGraph
	ID    program.NodeID
}

// refOf is the reference to node in graph; the zero id where graph does not index node.
func refOf(graph *lower.ActionGraph, node ast.Node) NodeRef {
	if graph == nil {
		return NodeRef{}
	}
	return NodeRef{Graph: graph, ID: graph.ID[node]}
}

// refsOf references each of nodes in graph, in order.
func refsOf(graph *lower.ActionGraph, nodes []ast.Node) []NodeRef {
	refs := make([]NodeRef, len(nodes))
	for i, node := range nodes {
		refs[i] = refOf(graph, node)
	}
	return refs
}

// Origin is where the node the reference names was written; the zero Origin for none.
func (r NodeRef) Origin() program.Origin {
	if r.Graph == nil || int(r.ID) >= len(r.Graph.Origins) {
		return program.Origin{}
	}
	return r.Graph.Origins[r.ID]
}

// Node is the declaration the reference names, nil for none.
func (r NodeRef) Node() ast.Node {
	if r.Graph == nil {
		return nil
	}
	return r.Graph.Provenance.Node(r.ID)
}

// NodesOf is the declarations the references name, in order.
func NodesOf(refs []NodeRef) []ast.Node {
	if refs == nil {
		return nil
	}
	nodes := make([]ast.Node, len(refs))
	for i, ref := range refs {
		nodes[i] = ref.Node()
	}
	return nodes
}

// BreakpointAt resolves a breakpoint on node, in the flow of the nested action nodes within
// (outermost first), to ids; false where a node is not one a performance of the one before holds.
func (e *ActionExecutor) BreakpointAt(within []ast.Node, node ast.Node) (NodeBreakpoint, bool) {
	refs := make([]NodeRef, 0, len(within)+1)
	for i, n := range append(slices.Clone(within), node) {
		var ref NodeRef
		var ok bool
		if i == 0 {
			ref, ok = locateNode(e.graph, n)
		} else {
			ref, ok = nestedRef(refs[i-1].Graph, within[i-1], n)
		}
		if !ok {
			return NodeBreakpoint{}, false
		}
		refs = append(refs, ref)
	}
	return NodeBreakpoint{Within: refs[:len(within)], Node: refs[len(within)]}, true
}

// nestedRef references node among those a performance of owner, a node of flow, holds as
// its own: the nodes of the flow owner owns (a deferred one unfolded), or of its body's block flows.
func nestedRef(flow *lower.ActionGraph, owner, node ast.Node) (NodeRef, bool) {
	if sub := flow.Subflows[owner]; sub != nil {
		if graph, err := sub.Unfold(); err == nil {
			if ref, ok := locateNode(graph, node); ok {
				return ref, true
			}
		}
	}
	if block := lower.NestedFlow(flow, owner, node); block != nil {
		return refOf(block, node), true
	}
	return NodeRef{}, false
}

// locateNode references node where a performance of flow runs it: as a node of flow's
// own, or of a block flow the body of one of its statement nodes states.
func locateNode(flow *lower.ActionGraph, node ast.Node) (NodeRef, bool) {
	if flow == nil {
		return NodeRef{}, false
	}
	if id, ok := flow.ID[node]; ok {
		return NodeRef{Graph: flow, ID: id}, true
	}
	for _, owner := range flow.Nodes {
		if _, isUsage := owner.(*ast.Usage); isUsage || !slices.Contains(flow.BlockNodes[owner], node) {
			continue
		}
		if block := lower.NestedFlow(flow, owner, node); block != nil {
			return refOf(block, node), true
		}
	}
	return NodeRef{}, false
}

// blockNodeRefs references the action nodes the blocks among stmts declare, each in
// the block flow it is a node of: lower.BlockNodes(stmts) by reference.
func blockNodeRefs(stmts []lower.Statement) []NodeRef {
	var refs []NodeRef
	for _, flow := range lower.BlockFlows(stmts) {
		for _, node := range flow.Nodes {
			if _, isUsage := node.(*ast.Usage); isUsage && !flow.StatementRuns[node] {
				refs = append(refs, refOf(flow, node))
				continue
			}
			refs = append(refs, blockNodeRefs(flow.Bodies[node])...)
		}
	}
	return refs
}

// flowNodeRefs references the nodes of graph: its own, then those the blocks of its
// statement nodes declare.
func flowNodeRefs(graph *lower.ActionGraph) []NodeRef {
	refs := refsOf(graph, graph.Nodes)
	for _, node := range graph.Nodes {
		if _, isUsage := node.(*ast.Usage); isUsage {
			continue
		}
		refs = append(refs, blockNodeRefs(graph.Bodies[node])...)
	}
	return refs
}

// usageRefsNamed returns the usages among refs named name.
func usageRefsNamed(refs []NodeRef, name string) []NodeRef {
	var named []NodeRef
	for _, ref := range refs {
		if _, isUsage := ref.Node().(*ast.Usage); isUsage && slices.Contains(actionNodeNames(ref.Node()), name) {
			named = append(named, ref)
		}
	}
	return named
}

// sortedRefs orders the keys of a table by the nodes they name, as a spelling lists them.
func sortedRefs[V any](m map[NodeRef]V) []NodeRef {
	refs := make([]NodeRef, 0, len(m))
	for ref := range m {
		refs = append(refs, ref)
	}
	slices.SortFunc(refs, func(a, b NodeRef) int { return strings.Compare(nodeKey(a.Node()), nodeKey(b.Node())) })
	return refs
}
