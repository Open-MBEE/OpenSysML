package lower

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// An action node whose own members state a flow — `first`, a succession, a fork
// — owns that flow rather than being a leaf: KerML makes its steps
// subperformances of it (`Actions::Action::subactions :> actions,
// subperformances`, `Performances::subperformances subsets suboccurrences`), and
// a suboccurrence is time-enclosed by the occurrence owning it. Such a node is
// lowered to a subgraph the executor runs to completion before the node's own
// succession fires.

// Subflow is the flow a nested action node's own members state. Graph is nil
// when those members state a flow that could not be built, which Err says; the
// executor reports it at initialize() rather than treating the node as a leaf.
type Subflow struct {
	Graph *ActionGraph
	Err   error
}

// PerformsLeafStatements reports whether a nested action node's members are a
// leaf body performing statements: they state no flow of their own
// (runsOwnFlow) and write a statement directly among them.
func PerformsLeafStatements(members []ast.Node) bool {
	if runsOwnFlow(members) {
		return false
	}
	for _, member := range members {
		switch unwrapMembership(member).(type) {
		case *ast.WhileLoopActionNode, *ast.IfActionNode, *ast.AssignmentActionNode,
			*ast.SendStatement, *ast.TerminateStatement:
			return true
		}
	}
	return false
}

// statesOwnFlow reports whether an action node's members state a flow of its
// own: a start, an end, an edge or a control node. A node whose members are only
// statements, parameters or undirected declarations states none and stays a leaf.
func statesOwnFlow(members []ast.Node) bool {
	for _, member := range members {
		if outsideBlockFlow(unwrapMembership(member)) {
			return true
		}
	}
	return false
}

// runsOwnFlow reports whether a nested action node runs a flow of its own: its
// members state one (statesOwnFlow) or declare a composite subaction, which is
// performed during the node's performance (Actions.sysml `subactions`).
func runsOwnFlow(members []ast.Node) bool {
	if statesOwnFlow(members) {
		return true
	}
	for _, member := range members {
		if usage, ok := unwrapMembership(member).(*ast.Usage); ok && startsConcurrently(usage) {
			return true
		}
	}
	return false
}

// lowerActionNode records what a nested action node runs: the flow its own
// members state (its start inferred as a whole action's is), or — where they
// state none — the statements and accept of a leaf. scope is the node's own namespace.
func lowerActionNode(graph *ActionGraph, node *ast.Usage, scope *symbols.Scope) {
	members, cycle, err := effectiveActionMembers(node, scope)
	if err != nil {
		recordInvalidSubflow(graph, node, err)
		return
	}
	if cycle {
		recordInvalidSubflow(graph, node, fmt.Errorf("%w: %s", ErrCyclicSpecialization, getNodeName(node)))
		return
	}
	lowerEffectiveActionNodeFeatures(graph, node, scope, members)
	lowerEffectiveAccept(graph, node, members)
	if node.IsTerminate {
		lowerTerminateNode(graph, node, scope, members)
		return
	}
	rawMembers := make([]ast.Node, 0, len(members))
	for _, member := range members {
		rawMembers = append(rawMembers, member.Decl)
	}
	typedBody, typedAction := mergedTypedActionBody(node, scope)
	var typedTarget ast.Node
	if typedBody {
		typedTarget = resolveTypedActionTarget(graph.resolver, typedAction)
		if typedActionTargetIsAncestor(graph, typedTarget) {
			if typedBodyHasExecutableContent(rawMembers) {
				recordInvalidSubflow(graph, node, fmt.Errorf("%w: %s",
					ErrRecursiveActionTyping, ast.SimpleName(typedAction.Target)))
				return
			}
			typedBody = false
		}
	}
	if !runsOwnFlow(rawMembers) && !typedBody {
		for _, member := range BodyStatementMembers(rawMembers) {
			actual := unwrapMembership(member)
			memberScope := effectiveMemberScope(members, actual, scope)
			graph.recordDeclaredIn(actual, memberScope)
			graph.Bodies[node] = append(graph.Bodies[node], lowerStatement(actual, memberScope))
		}
		if _, _, starts := startedBehavior(node, scope); starts {
			graph.Bodies[node] = append(graph.Bodies[node], performEffect(node, scope))
		}
		return
	}
	if graph.Subflows == nil {
		graph.Subflows = make(map[ast.Node]*Subflow)
	}
	ancestors := graph.lowering
	if typedBody {
		ancestors = appendLoweringAncestor(ancestors, typedTarget)
	}
	sub, err := toActionGraphWithTypingAndAncestors(node, scope, graph.resolver, typedBody, ancestors)
	if err == nil {
		sub.Enclosing, sub.EnclosingNode = graph, node
		StartFlow(sub)
	}
	graph.Subflows[node] = &Subflow{Graph: sub, Err: err}
	if typedBody && typedAction.Target != nil {
		if graph.MergedTypedSubflows == nil {
			graph.MergedTypedSubflows = make(map[ast.Node]PerformedType)
		}
		graph.MergedTypedSubflows[node] = typedAction
	}
}

func resolveTypedActionTarget(resolver *resolve.Resolver, typed PerformedType) ast.Node {
	if typed.Target == nil || typed.Scope == nil {
		return nil
	}
	if resolver != nil {
		if decl, _, ok := resolver.TypeDecl(typed.Scope, typed.Target); ok {
			return decl
		}
	}
	decl, _, _ := resolve.TypeDeclInScope(typed.Scope, typed.Target)
	return decl
}

func typedActionTargetIsAncestor(graph *ActionGraph, target ast.Node) bool {
	if graph == nil {
		return false
	}
	return loweringHasAncestor(graph.lowering, target)
}

func loweringHasAncestor(ancestors []ast.Node, target ast.Node) bool {
	if target == nil {
		return false
	}
	for _, ancestor := range ancestors {
		if ancestor == target {
			return true
		}
	}
	return false
}

func appendLoweringAncestor(ancestors []ast.Node, target ast.Node) []ast.Node {
	if target == nil || loweringHasAncestor(ancestors, target) {
		return ancestors
	}
	out := append([]ast.Node(nil), ancestors...)
	return append(out, target)
}

func typedBodyHasExecutableContent(members []ast.Node) bool {
	for _, member := range members {
		actual := unwrapMembership(member)
		if actual == nil || statesNoStep(actual) || isAnnotation(actual) {
			continue
		}
		switch node := actual.(type) {
		case *ast.Usage:
			switch node.Kind {
			case ast.UsageAction, ast.UsageState, ast.UsageTransition, ast.UsageSuccession,
				ast.UsageStep, ast.UsageAnalysisCase, ast.UsageVerificationCase:
				return true
			default:
				if node.IsAccept || node.IsTerminate || node.IsActionNode {
					return true
				}
			}
		case *ast.AcceptActionUsage, *ast.EntryMember, *ast.DoMember, *ast.ExitMember,
			*ast.StateNode:
			return true
		default:
			return true
		}
	}
	return false
}

func mergedTypedActionBody(node *ast.Usage, scope *symbols.Scope) (bool, PerformedType) {
	if node == nil || !node.HasBody || declaresOnlyFeatures(node.Members) {
		return false, PerformedType{}
	}
	for _, rel := range node.Relationships {
		if rel == nil || (rel.Kind != ast.RelTyping && rel.Kind != ast.RelReferences) {
			continue
		}
		target, ok := rel.Target.(*ast.QualifiedName)
		if !ok {
			continue
		}
		declaringScope := scope
		if declaringScope != nil && declaringScope.Parent() != nil {
			declaringScope = declaringScope.Parent()
		}
		return true, PerformedType{Target: target, Scope: declaringScope}
	}
	return false, PerformedType{}
}

type effectiveActionMember struct {
	Decl  ast.Node
	Scope *symbols.Scope
}

func effectiveActionMembers(node *ast.Usage, scope *symbols.Scope) ([]effectiveActionMember, bool, error) {
	owner := scope
	if owner != nil && owner.Parent() != nil {
		owner = owner.Parent()
	}
	var collect func(*ast.Usage, *symbols.Scope, map[ast.Node]bool) ([]effectiveActionMember, bool, error)
	collect = func(usage *ast.Usage, bodyScope *symbols.Scope, active map[ast.Node]bool) ([]effectiveActionMember, bool, error) {
		if active[usage] {
			return nil, true, nil
		}
		active[usage] = true
		defer delete(active, usage)
		declaringBody := bodyScope
		if declaringBody != nil && declaringBody.Parent() != nil {
			declaringBody = declaringBody.Parent()
		}
		if missing := resolve.MissingRedefinedActionNode(declaringBody, usage); missing != "" {
			return nil, false, fmt.Errorf("%w: %s", ErrRedefinedStepMissing, missing)
		}
		var out []effectiveActionMember
		seen := make(map[ast.Node]bool)
		for _, member := range usage.Members {
			decl := unwrapMembership(member)
			out = append(out, effectiveActionMember{Decl: decl, Scope: bodyScope})
			seen[decl] = true
		}
		for _, target := range resolve.ActionNodeRedefinitionTargets(declaringBody, usage, true) {
			general, ok := target.(*ast.Usage)
			if !ok || general.Kind != ast.UsageAction {
				continue
			}
			generalOwner := resolve.ActionNodeDeclaringScope(declaringBody, target)
			if generalOwner == nil {
				continue
			}
			generalScope := childScope(generalOwner, general)
			inherited, cyclic, err := collect(general, generalScope, active)
			if err != nil {
				return out, false, err
			}
			if cyclic {
				return out, true, nil
			}
			for _, member := range inherited {
				replaced := false
				for _, own := range usage.Members {
					redefinition, ok := unwrapMembership(own).(*ast.Usage)
					if ok && resolve.RedefinesActionNode(declaringBody, redefinition, member.Decl) {
						replaced = true
						break
					}
				}
				if !replaced && !seen[member.Decl] {
					seen[member.Decl] = true
					out = append(out, member)
				}
			}
		}
		return out, false, nil
	}
	members, cycle, err := collect(node, scope, make(map[ast.Node]bool))
	return members, cycle, err
}

func effectiveMemberScope(members []effectiveActionMember, decl ast.Node, fallback *symbols.Scope) *symbols.Scope {
	for _, member := range members {
		if member.Decl == decl {
			return member.Scope
		}
	}
	return fallback
}

func recordInvalidSubflow(graph *ActionGraph, node ast.Node, err error) {
	if graph.Subflows == nil {
		graph.Subflows = make(map[ast.Node]*Subflow)
	}
	graph.Subflows[node] = &Subflow{Err: err}
}

// lowerTerminateNode records what a terminate action usage runs: the statements of
// its body as a leaf's, then the terminate it stands for. A body stating a flow of
// its own has no place to end the performance from, so it is refused at initialize.
func lowerTerminateNode(graph *ActionGraph, node *ast.Usage, scope *symbols.Scope, members []effectiveActionMember) {
	rawMembers := make([]ast.Node, 0, len(members))
	for _, member := range members {
		rawMembers = append(rawMembers, member.Decl)
	}
	if statesOwnFlow(rawMembers) {
		if graph.Subflows == nil {
			graph.Subflows = make(map[ast.Node]*Subflow)
		}
		graph.Subflows[node] = &Subflow{Err: errors.New("a terminate action usage states no flow of its own")}
		return
	}
	for _, member := range BodyStatementMembers(rawMembers) {
		actual := unwrapMembership(member)
		memberScope := effectiveMemberScope(members, actual, scope)
		graph.recordDeclaredIn(actual, memberScope)
		graph.Bodies[node] = append(graph.Bodies[node], lowerStatement(actual, memberScope))
	}
	graph.Bodies[node] = append(graph.Bodies[node], lowerStatement(node, scope))
}

// TerminateUsage returns the terminate a terminate action usage stands for, the last of
// its node's body after the statements it declares; false for a node that is none.
func (g *ActionGraph) TerminateUsage(node ast.Node) (Effect, bool) {
	body := g.Bodies[node]
	if len(body) == 0 {
		return Effect{}, false
	}
	last, ok := body[len(body)-1].(Effect)
	if !ok || last.Kind != EffectTerminate || last.Terminates != TerminateEnclosing || last.Node != node {
		return Effect{}, false
	}
	return last, true
}

// StartUsage returns the start a performed action node stands for (`perform
// obj.beh.start;`), the last of its body; false for a node that performs otherwise.
func (g *ActionGraph) StartUsage(node ast.Node) (Effect, bool) {
	body := g.Bodies[node]
	if len(body) == 0 {
		return Effect{}, false
	}
	last, ok := body[len(body)-1].(Effect)
	if !ok || last.Kind != EffectStart || last.Node != node {
		return Effect{}, false
	}
	return last, true
}

// lowerEffectiveAccept records the message a nested action node waits for, which
// a node owning a flow still does before that flow starts.
func lowerEffectiveAccept(graph *ActionGraph, node *ast.Usage, members []effectiveActionMember) {
	for _, member := range members {
		m, ok := member.Decl.(*ast.Usage)
		if !ok || !m.IsAccept {
			continue
		}
		scope := member.Scope
		if scope == nil {
			scope = graph.nodeScope(node)
		}
		port, viaSelf := acceptPort(node)
		graph.Accepts[node] = Accept{
			ParamName:    m.Ident.Name,
			SignalType:   typingTarget(m),
			ViaPort:      port,
			ViaSelf:      viaSelf,
			SubsetsEvent: subsettingTarget(m),
			Trigger:      m.Value,
			Scope:        scope,
			Keeper:       IsDeferredKeeper(graph.resolver, scope, node),
		}
	}
}
