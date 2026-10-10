package lower

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func actionDescription(scope *symbols.Scope) string {
	if scope != nil {
		if name := ast.SimpleName(scope.Node()); name != "" {
			return name
		}
	}
	return "action"
}

func mergeInheritedActionContent(graph *ActionGraph, bodies []*symbols.Scope) ([]pendingGate, error) {
	if graph == nil {
		return nil, nil
	}
	ordered := orderedAssertionsForGraph(graph, bodies)
	incompatible := make(map[ast.Node]ast.Node)
	allBodies := append([]*symbols.Scope{graph.Scope}, bodies...)
	for _, body := range allBodies {
		if body == nil || body.Node() == nil {
			continue
		}
		for _, member := range ast.DeclMembers(body.Node()) {
			decl := unwrapMembership(member)
			if lowerableActionNodeInBody(decl, ordered) {
				continue
			}
			for _, target := range resolve.ActionNodeRedefinitionTargets(body, decl, false) {
				incompatible[target] = decl
			}
		}
	}
	graph.replacedByIncompatible = incompatible

	seenBodies := make(map[*symbols.Scope]bool)
	for _, body := range bodies {
		if body == nil || seenBodies[body] {
			continue
		}
		seenBodies[body] = true
		if !hasInheritedBody(graph, body) {
			graph.inherited = append(graph.inherited, Inherited{Decl: body.Node(), Body: body})
		}
		for _, member := range ast.DeclMembers(body.Node()) {
			decl := unwrapMembership(member)
			if !lowerableActionNodeInBody(decl, ordered) {
				continue
			}
			if initial, marker := decl.(*ast.InitialNode); marker {
				graph.recordDeclaredIn(initial, body)
				continue
			}
			if final, ok := decl.(*ast.FinalNode); ok {
				ensureInheritedDone(graph, final, body)
				continue
			}
			matches := inheritedNodeMatches(graph, body, decl)
			if len(matches) > 1 {
				return nil, fmt.Errorf("%w: %s", ErrAmbiguousInheritedStep, ast.SimpleName(decl))
			}
			if len(matches) == 1 {
				recordInheritedPerform(graph, matches[0], decl, body)
				continue
			}
			if replacement := incompatible[decl]; replacement != nil {
				continue
			}
			if shadowsInheritedActionNode(graph, body, decl) {
				continue
			}
			ensureDeclaredActionNode(graph, decl, body)
		}
	}

	for _, body := range bodies {
		if body == nil {
			continue
		}
		for _, member := range ast.DeclMembers(body.Node()) {
			decl := unwrapMembership(member)
			if !lowerableActionNodeInBody(decl, ordered) {
				continue
			}
			if _, marker := decl.(*ast.InitialNode); marker {
				continue
			}
			if _, marker := decl.(*ast.FinalNode); marker {
				continue
			}
			if len(inheritedNodeMatches(graph, body, decl)) > 1 {
				return nil, fmt.Errorf("%w: %s", ErrAmbiguousInheritedStep, ast.SimpleName(decl))
			}
		}
	}

	for _, body := range bodies {
		if body == nil || graph.Initial != nil {
			continue
		}
		for _, member := range ast.DeclMembers(body.Node()) {
			initial, ok := unwrapMembership(member).(*ast.InitialNode)
			if !ok || initial.Successor != nil {
				continue
			}
			if node := inheritedNodeLookup(graph, body)(ast.SimpleName(initial.First)); node != nil {
				graph.Initial = node
				graph.recordDeclaredIn(initial, body)
				break
			}
			if ast.SimpleName(initial.First) == "start" {
				graph.Initial = resolveActionEndpoint(graph, initial.First, true)
				graph.recordDeclaredIn(initial, body)
				break
			}
		}
	}

	seenEdges := make(map[ast.Node]bool)
	var gates []pendingGate
	for _, body := range bodies {
		if body == nil {
			continue
		}
		lookup := inheritedNodeLookup(graph, body)
		lowerer := &actionEdgeLowerer{
			graph:        graph,
			scope:        body,
			weights:      &probabilityReader{resolver: graph.resolver, scope: body},
			nodes:        lookup,
			incompatible: incompatible,
		}
		for _, member := range ast.DeclMembers(body.Node()) {
			decl := unwrapMembership(member)
			if !inheritedActionEdge(decl) || seenEdges[decl] {
				continue
			}
			seenEdges[decl] = true
			graph.recordDeclaredIn(decl, body)
			if err := lowerer.member(decl); err != nil {
				return nil, err
			}
		}
		gates = append(gates, lowerer.gates...)
	}
	return gates, nil
}

func recordInheritedPerform(graph *ActionGraph, replacement, redefined ast.Node, declaringScope *symbols.Scope) {
	node, ok := redefined.(*ast.Usage)
	if !ok || !node.Kind.IsAction() || graph == nil {
		return
	}
	target := typingTarget(node)
	if target == nil {
		return
	}
	usage, ok := replacement.(*ast.Usage)
	if !ok || !usage.Kind.IsAction() || typingTarget(usage) != nil {
		return
	}
	if graph.Performs == nil {
		graph.Performs = make(map[ast.Node]PerformedType)
	}
	if _, recorded := graph.Performs[replacement]; !recorded {
		graph.Performs[replacement] = PerformedType{Target: target, Scope: declaringScope}
	}
}

func hasInheritedBody(graph *ActionGraph, body *symbols.Scope) bool {
	for _, inherited := range graph.inherited {
		if inherited.Body == body {
			return true
		}
	}
	return false
}

func lowerableActionNode(decl ast.Node) bool {
	switch n := decl.(type) {
	case *ast.Usage:
		return n.Kind.IsAction() || IsCaseNode(n)
	case *ast.InitialNode, *ast.FinalNode, *ast.ForkNode, *ast.JoinNode,
		*ast.MergeNode, *ast.DecisionNode, *ast.ActionExecutionNode,
		*ast.PerformActionNode, *ast.WhileLoopActionNode, *ast.IfActionNode,
		*ast.AssignmentActionNode, *ast.SendStatement, *ast.TerminateStatement:
		return true
	default:
		return false
	}
}

func orderedAssertionsForGraph(graph *ActionGraph, bodies []*symbols.Scope) map[*ast.Usage]bool {
	members := make([]ast.Node, 0)
	seen := make(map[ast.Node]bool)
	appendBody := func(body *symbols.Scope) {
		if body == nil || body.Node() == nil {
			return
		}
		for _, member := range ast.DeclMembers(body.Node()) {
			decl := unwrapMembership(member)
			if decl == nil || seen[decl] {
				continue
			}
			seen[decl] = true
			members = append(members, decl)
		}
	}
	appendBody(graph.Scope)
	for _, body := range bodies {
		appendBody(body)
	}
	return orderedAssertions(members)
}

func lowerableActionNodeInBody(decl ast.Node, ordered map[*ast.Usage]bool) bool {
	assertion, ok := decl.(*ast.Usage)
	if !ok || !resolve.IsAssertion(assertion) {
		return lowerableActionNode(decl)
	}
	return ordered[assertion]
}

func inheritedActionEdge(decl ast.Node) bool {
	switch n := decl.(type) {
	case *ast.InitialNode:
		return n.Successor != nil
	case *ast.SuccessionEdge, *ast.ControlFlowEdge, *ast.TransitionMember, *ast.ObjectFlowEdge:
		return true
	case *ast.Usage:
		return n.Kind == ast.UsageSuccession
	default:
		return false
	}
}

func inheritedNodeMatches(graph *ActionGraph, body *symbols.Scope, decl ast.Node) []ast.Node {
	var matches []ast.Node
	seen := make(map[ast.Node]bool)
	for _, node := range graph.Nodes {
		matchesNode := node == decl
		if usage, ok := node.(*ast.Usage); ok && !matchesNode {
			owner := graph.Scope
			if scope := graph.Scopes[node]; scope != nil && scope.Parent() != nil {
				owner = scope.Parent()
			}
			matchesNode = resolve.RedefinesActionNode(owner, usage, decl)
		}
		if matchesNode && !seen[node] {
			seen[node] = true
			matches = append(matches, node)
		}
	}
	return matches
}

func shadowsInheritedActionNode(graph *ActionGraph, body *symbols.Scope, decl ast.Node) bool {
	name := getNodeName(decl)
	if name == "" {
		return false
	}
	for _, node := range graph.Nodes {
		if node == decl || !nodeAnswersTo(node, name) {
			continue
		}
		owner := graph.Scope
		if scope := graph.Scopes[node]; scope != nil && scope.Parent() != nil {
			owner = scope.Parent()
		}
		if owner == nil {
			continue
		}
		generals, _ := resolve.ActionGeneralization(owner, true)
		for _, general := range generals {
			if general == body {
				return true
			}
		}
	}
	return false
}

func ensureInheritedDone(graph *ActionGraph, decl *ast.FinalNode, body *symbols.Scope) ast.Node {
	graph.recordDeclaredIn(decl, body)
	if graph.doneMarker != nil {
		return graph.doneMarker
	}
	for _, final := range graph.Finals {
		if getNodeName(final) == "done" {
			graph.doneMarker = final
			return final
		}
	}
	final := &ast.FinalNode{NodeBase: ast.NodeBase{NodeSpan: decl.Span()}}
	graph.Finals = append(graph.Finals, final)
	graph.Nodes = append(graph.Nodes, final)
	graph.doneMarker = final
	graph.recordDeclaredIn(final, body)
	return final
}
