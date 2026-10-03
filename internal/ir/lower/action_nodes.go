package lower

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// ActionNodes returns the nodes accepted by action lowering and whether the
// action has an initial node after interpreting `first <node>`.
func ActionNodes(actionDecl ast.Node, scope *symbols.Scope) (nodes []ast.Node, hasInitial bool, err error) {
	members, err := actionMembers(actionDecl)
	if err != nil {
		return nil, false, err
	}
	graph, err := collectActionNodes(members, scope, nil)
	if err != nil {
		return nil, false, err
	}
	return graph.Nodes, graph.Initial != nil, nil
}

// ActionEndpointAccepted reports whether an action endpoint names a lowered
// node or one of the implicit start/done markers.
func ActionEndpointAccepted(nodes []ast.Node, hasInitial bool, ref ast.Node, source bool) bool {
	if findNodeByReference(nodes, ref) != nil {
		return true
	}
	return impliedMarker(ast.SimpleName(ref), source, !hasInitial)
}

func impliedMarker(name string, source, noInitial bool) bool {
	return (source && noInitial && name == "start") || (!source && name == "done")
}

// ToActionInterface lowers only the attributes an action declares, for a performance that
// runs no body of it: an external tool's. Its flow is empty whatever the body states.
func ToActionInterface(actionDecl ast.Node, scope *symbols.Scope) (*ActionGraph, error) {
	members, err := actionMembers(actionDecl)
	if err != nil {
		return nil, err
	}
	graph := newActionGraph(scope)
	graph.Attributes = lowerAttributes(members)
	return graph, nil
}

func newActionGraph(scope *symbols.Scope) *ActionGraph {
	return &ActionGraph{
		Scope:          scope,
		Nodes:          make([]ast.Node, 0),
		Edges:          make(map[ast.Node][]ActionEdge),
		Multiplicities: make(map[ast.Node]*ast.Multiplicity),
		DataFlows:      make(map[ast.Node][]ObjectFlow),
		Bodies:         make(map[ast.Node][]Statement),
		Accepts:        make(map[ast.Node]Accept),
		Finals:         make([]ast.Node, 0),
	}
}

// actionMembers is the body an action's declaration states.
func actionMembers(actionDecl ast.Node) ([]ast.Node, error) {
	switch n := actionDecl.(type) {
	case *ast.Usage:
		return n.Members, nil
	case *ast.Definition:
		return n.Members, nil
	default:
		return nil, fmt.Errorf("action must be Usage or Definition, got %T", actionDecl)
	}
}

func collectActionNodes(members []ast.Node, scope *symbols.Scope, resolver *resolve.Resolver) (*ActionGraph, error) {
	graph := newActionGraph(scope)
	graph.resolver = resolver
	asserted := orderedAssertions(members)

	// First pass: collect nodes.
	for _, member := range members {
		actualMember := unwrapMembership(member)

		switch n := actualMember.(type) {
		case *ast.InitialNode:
			// `first a then b;` states the succession a -> b and declares no node.
			if n.Successor != nil {
				continue
			}
			if graph.Initial != nil {
				return graph, fmt.Errorf("action has multiple initial nodes")
			}
			graph.Initial = n
			graph.Nodes = append(graph.Nodes, n)
			lowerNodeBody(graph, n, n.Members, scope)
		case *ast.FinalNode:
			graph.Finals = append(graph.Finals, n)
			graph.Nodes = append(graph.Nodes, n)
		case *ast.ForkNode, *ast.JoinNode, *ast.MergeNode, *ast.DecisionNode, *ast.ActionExecutionNode:
			graph.Nodes = append(graph.Nodes, n)
			lowerNodeBody(graph, n, ast.NodeBodyMembers(n), scope)
		case *ast.Usage:
			switch {
			case n.Kind == ast.UsageAction:
				graph.Nodes = append(graph.Nodes, n)
				recordNodeMultiplicity(graph, n)
				lowerActionNode(graph, n, childScope(scope, n))
			case IsCaseNode(n):
				graph.Nodes = append(graph.Nodes, n)
				recordNodeMultiplicity(graph, n)
				recordNodeScope(graph, n, childScope(scope, n))
			case asserted[n]:
				graph.Nodes = append(graph.Nodes, n)
				graph.Bodies[n] = []Statement{Assert{Node: n, Sym: scope.MemberDeclaring(n), Scope: scope}}
			}
		case *ast.WhileLoopActionNode, *ast.IfActionNode, *ast.AssignmentActionNode,
			*ast.SendStatement, *ast.TerminateStatement:
			// A statement written among the action's own members is a subaction of
			// it, ordered by the successions that bind it and started with the
			// owner where none does (StartFlow).
			graph.Nodes = append(graph.Nodes, n)
			graph.Bodies[n] = []Statement{lowerStatement(n, scope)}
		}
	}

	collectInheritedActionNodes(graph, members)
	graph.Connections = lowerConnections(members, OwnerBehavior, scope)
	graph.Attributes = lowerAttributes(members)
	// `first a;` names the node the flow starts at rather than declaring one.
	if err := resolveFirstNode(graph); err != nil {
		return nil, err
	}
	return graph, nil
}

func collectInheritedActionNodes(graph *ActionGraph, members []ast.Node) {
	for _, member := range members {
		switch n := unwrapMembership(member).(type) {
		case *ast.InitialNode:
			if n.Successor != nil {
				ensureInheritedActionNode(graph, n.First)
				ensureInheritedActionNode(graph, n.Successor)
			}
		case *ast.SuccessionEdge:
			if n.SourceMember == nil && !n.SourceImplied {
				ensureInheritedActionNode(graph, n.Source)
			}
			if n.TargetMember == nil && !n.TargetImplied {
				ensureInheritedActionNode(graph, n.Target)
			}
		case *ast.ControlFlowEdge:
			if n.SourceMember == nil && !n.SourceImplied {
				ensureInheritedActionNode(graph, n.Source)
			}
			if n.TargetMember == nil && !n.TargetImplied {
				ensureInheritedActionNode(graph, n.Target)
			}
		case *ast.TransitionMember:
			if n.Source != nil {
				ensureInheritedActionNode(graph, n.Source)
			}
			ensureInheritedActionNode(graph, n.Target)
		case *ast.Usage:
			if n.Kind == ast.UsageSuccession && len(n.ConnectorEnds) == 2 {
				ensureInheritedActionNode(graph, connectorEndReference(n.ConnectorEnds[0]))
				ensureInheritedActionNode(graph, connectorEndReference(n.ConnectorEnds[1]))
			}
		}
	}
}

func ensureInheritedActionNode(graph *ActionGraph, ref ast.Node) ast.Node {
	qn := actionEndpointQualifiedName(ref)
	if qn == nil {
		return nil
	}
	decl, declaringScope, found, _ := resolve.ActionNodeInScope(graph.Scope, qn)
	if !found || decl == nil {
		return nil
	}
	for _, node := range graph.Nodes {
		if node == decl {
			return node
		}
	}
	graph.Nodes = append(graph.Nodes, decl)
	graph.recordDeclaredIn(decl, declaringScope)
	switch n := decl.(type) {
	case *ast.Usage:
		if resolve.IsAssertion(n) {
			graph.Bodies[n] = []Statement{Assert{Node: n, Sym: declaringScope.MemberDeclaring(n), Scope: declaringScope}}
			break
		}
		recordNodeMultiplicity(graph, n)
		lowerActionNode(graph, n, childScope(declaringScope, n))
	case *ast.ForkNode, *ast.JoinNode, *ast.MergeNode, *ast.DecisionNode, *ast.ActionExecutionNode:
		lowerNodeBody(graph, n, ast.NodeBodyMembers(n), declaringScope)
	case *ast.WhileLoopActionNode, *ast.IfActionNode, *ast.AssignmentActionNode,
		*ast.SendStatement, *ast.TerminateStatement:
		graph.Bodies[n] = []Statement{lowerStatement(n, declaringScope)}
	}
	return decl
}

func actionEndpointQualifiedName(ref ast.Node) *ast.QualifiedName {
	if qn := ast.AsQualifiedName(ref); qn != nil {
		return qn
	}
	chain, ok := ref.(*ast.FeatureChainExpr)
	if !ok {
		return nil
	}
	return actionEndpointQualifiedName(chain.Operand)
}
