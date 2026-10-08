package lower

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A flow's composite subactions are its owner's subperformances (Performances.kerml:
// `composite step subperformances: Performance[0..*] subsets enclosedPerformances,
// suboccurrences`; Actions.sysml: `action subactions: Action[0..*] :> actions,
// subperformances`), each happening during the owner's performance
// (`enclosedPerformances ... subsets timeEnclosedOccurrences`). Only a succession
// (`HappensBefore`) orders one subaction after another, so a subaction no
// succession leads to is performed from the start of its owner's performance,
// unordered against every other such subaction and against the flow `first` starts.

// Starts returns the nodes a performance of the flow starts at: Initial, where the
// flow has one, then each concurrent start in declaration order.
func (g *ActionGraph) Starts() []ast.Node {
	if g == nil {
		return nil
	}
	starts := make([]ast.Node, 0, 1+len(g.Concurrent))
	if g.Initial != nil {
		starts = append(starts, g.Initial)
	}
	return append(starts, g.Concurrent...)
}

// FlowStartError reports a flow stating steps that nothing can start: one whose
// successions form a cycle over every step, so no step is unpreceded. A flow with
// a start, one stating no step, and one whose only unpreceded steps are not
// composite subactions (`ref` and `perform` usages, control nodes, which are
// performed only where a succession reaches them) have none.
func FlowStartError(graph *ActionGraph) error {
	if graph == nil || len(graph.Starts()) > 0 {
		return nil
	}
	preceded := precededNodes(graph)
	steps := false
	for _, node := range graph.Nodes {
		if _, final := node.(*ast.FinalNode); final {
			continue
		}
		steps = true
		if !preceded[node] {
			return nil
		}
	}
	if !steps {
		return nil
	}
	return fmt.Errorf("the successions form a cycle among the steps, leaving none to start at")
}

// performedStep reports a node a flow may start at where nothing precedes it:
// anything but a `ref` or abstract usage no succession leaves, which is no step of
// the owner's performance (startsConcurrently) nor of the flow it states. A sole
// `perform` performs the behavior it names.
func performedStep(graph *ActionGraph, node ast.Node) bool {
	if graph.StatementRuns[node] {
		return true
	}
	usage, ok := node.(*ast.Usage)
	if !ok || len(graph.Edges[node]) > 0 || usage.IsPerformedAction() {
		return true
	}
	return semantics.UsageDeclIsComposite(usage) && !usage.IsAbstract
}

// unorderedSubactions returns the composite subactions of graph that no succession
// leads to, Initial aside, in declaration order.
func unorderedSubactions(graph *ActionGraph) []ast.Node {
	preceded := precededNodes(graph)
	targets := specializedSiblings(graph)
	var starts []ast.Node
	for _, node := range graph.Nodes {
		if node == graph.Initial || preceded[node] || !graph.StatementRuns[node] && !startsConcurrently(node) {
			continue
		}
		// A sibling subsetting or redefining the node performs it: its performance
		// is one of the node's own, so the node is not performed a second time.
		if name := getNodeName(node); name != "" && targets[name] {
			continue
		}
		starts = append(starts, node)
	}
	return starts
}

// startsConcurrently reports whether node is a composite subaction its owner
// performs whether or not a succession reaches it: an owned composite action usage
// (an `accept`, a terminate usage, a nested case or analysis included) or a send,
// assignment, if, loop or terminate action usage written as a body member, each a
// composite subaction (Actions.sysml `sendSubactions`, `acceptSubactions`,
// `assignments`, `ifSubactions`, `loops`, `terminateSubactions`). A `ref` usage, a
// `perform` (an EventOccurrenceUsage, which SysML v2 §8.3.16 requires to be
// referential), a parameter, an abstract usage, a control node and the start and
// final nodes are not.
func startsConcurrently(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.Usage:
		if n.Kind != ast.UsageAction && !IsCaseNode(n) {
			return false
		}
		return semantics.UsageDeclIsComposite(n) && !n.IsAbstract && !n.IsPerformedAction() && !n.IsBodyParameter
	case *ast.WhileLoopActionNode, *ast.IfActionNode, *ast.AssignmentActionNode,
		*ast.SendStatement, *ast.TerminateStatement:
		return true
	}
	return false
}

// precededNodes marks the nodes of graph some succession leads to.
func precededNodes(graph *ActionGraph) map[ast.Node]bool {
	preceded := make(map[ast.Node]bool, len(graph.Nodes))
	for _, edges := range graph.Edges {
		for _, edge := range edges {
			preceded[edge.Target] = true
		}
	}
	return preceded
}

// specializedSiblings names the nodes of graph another node of it subsets,
// redefines or is typed by.
func specializedSiblings(graph *ActionGraph) map[string]bool {
	targets := make(map[string]bool)
	for _, node := range graph.Nodes {
		usage, ok := node.(*ast.Usage)
		if !ok {
			continue
		}
		for _, rel := range usage.Relationships {
			if rel == nil {
				continue
			}
			switch rel.Kind {
			case ast.RelSubsets, ast.RelRedefines, ast.RelTyping:
				if name := ast.SimpleName(rel.Target); name != "" && name != usage.Ident.Name {
					targets[name] = true
				}
			}
		}
	}
	return targets
}
