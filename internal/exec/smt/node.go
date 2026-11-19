package smt

import (
	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
)

// Node names one node of a lowered flow: the graph it was lowered into and its id
// there. The zero Node is none.
type Node struct {
	Graph *lower.ActionGraph
	ID    program.NodeID
}

// initialOf is the node graph starts at, the zero Node's id for a graph with none.
func initialOf(graph *lower.ActionGraph) Node { return Node{Graph: graph, ID: graph.InitialID()} }

// at is the node of n's graph behind id.
func (n Node) at(id program.NodeID) Node { return Node{Graph: n.Graph, ID: id} }

// form is how the node was written.
func (n Node) form() lower.NodeForm { return n.Graph.FormOf(n.ID) }

// performs reports a usage standing as a node, which performs in a frame of its own.
func (n Node) performs() bool { return n.form() == lower.FormUsage }

// entry is the expression behind id in the node's graph, nil for none.
func (n Node) entry(id program.ExprID) *lower.ExprEntry { return exprOf(n.Graph, id) }

// exprOf is the expression behind id in graph's table, nil for none.
func exprOf(graph *lower.ActionGraph, id program.ExprID) *lower.ExprEntry {
	if id == 0 || graph.Exprs == nil {
		return nil
	}
	return graph.Exprs.Entry(id)
}

// nodeLabel names a node as a trace names it: by its name, else by what it does.
func nodeLabel(node Node) string {
	if node.Graph == nil {
		return "nil"
	}
	return node.Graph.TraceLabel(node.ID)
}
