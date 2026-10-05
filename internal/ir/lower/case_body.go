package lower

import (
	"fmt"
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// PerformsSteps reports whether decl is a behavior whose body performs the action
// nodes among its members as steps: an analysis case (SysML v2 §7.22) or a
// verification case (§7.23), each a calculation and an action. A calc's body
// reads them as declarations only.
func PerformsSteps(decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefAnalysisCase || d.Kind == ast.DefVerificationCase
	case *ast.Usage:
		return d.Kind == ast.UsageAnalysisCase || d.Kind == ast.UsageVerificationCase
	default:
		return false
	}
}

// caseSteps lowers a body whose steps are action nodes: its locals, one Block
// over the flow the steps state, then its results. A body stating successions or
// control nodes is the token flow an action body is (ToActionGraph); otherwise
// its action nodes and statements are unordered subactions.
func caseSteps(owner ast.Node, body []ast.Node, scope *symbols.Scope, resolver *resolve.Resolver) []Statement {
	trailing := trailingResults(body, scope)
	if !statesOwnFlow(body) {
		trailing = caseTrailingResults(body, scope, trailing)
		var locals, results []Statement
		var members []ast.Node
		statementRuns := make(map[ast.Node]Statement)
		flowStarted := false
		for _, member := range body {
			actual := unwrapMembership(member)
			if usage, ok := actual.(*ast.Usage); ok && caseFlowConnector(usage) {
				members = append(members, member)
				continue
			}
			if isFlowNode(actual) {
				members = append(members, member)
				flowStarted = true
				continue
			}
			stmt, states := calcStep(member, scope)
			if !states {
				members = append(members, member)
				continue
			}
			if isReturn(stmt) || trailing[member] || caseOutputBinding(actual) {
				results = append(results, stmt)
				continue
			}
			switch declared := stmt.(type) {
			case Declare:
				usage, _ := actual.(*ast.Usage)
				if declared.Value == nil || usage != nil && usage.ValueIsInitial || !flowStarted {
					locals = append(locals, stmt)
				} else {
					statementRuns[actual] = stmt
				}
			case DeclareUsage:
				locals = append(locals, stmt)
			default:
				members = append(members, member)
				flowStarted = true
			}
		}
		graph, err := lowerActionFlow(members, scope, resolver)
		if err != nil {
			return append(append(locals, Unsupported{
				Description: "the flow the steps of the body state: " + err.Error(),
				Node:        owner,
				Scope:       scope,
			}), results...)
		}
		addCasePerformNodes(graph, members, scope)
		addCaseStatementRuns(graph, body, statementRuns)
		graph.UnstatedCaseFlow = true
		StartFlow(graph)
		flow := Block{Node: owner, Scope: scope, Graph: graph, Own: true, Stated: true}
		return append(append(locals, flow), results...)
	}

	// The flow's nodes and the members sequencing them are the graph's; the
	// other members are the case's locals and results, as in a calc body. A
	// result no succession sequences ends the body after the flow, not in it.
	sequenced := sequencedMembers(body)
	stated := make([]ast.Node, 0, len(body))
	for _, member := range body {
		if !trailing[member] || sequenced[member] {
			stated = append(stated, member)
		}
	}
	graph, err := lowerActionFlow(stated, scope, resolver)
	var nodes map[ast.Node]bool
	if err == nil {
		nodes = make(map[ast.Node]bool, len(graph.Nodes))
		for _, node := range graph.Nodes {
			nodes[node] = true
		}
	}
	var locals, results []Statement
	for _, member := range body {
		if isFlowNode(member) || outsideBlockFlow(member) || sequenced[member] || nodes[unwrapMembership(member)] {
			continue
		}
		stmt, states := calcStep(member, scope)
		if !states {
			continue
		}
		if isReturn(stmt) || trailing[member] {
			results = append(results, stmt)
			continue
		}
		locals = append(locals, stmt)
	}
	if err != nil {
		unsupported := Unsupported{
			Description: "the flow the steps of the body state: " + err.Error(),
			Node:        owner,
			Scope:       scope,
		}
		return append(append(locals, unsupported), results...)
	}
	StartFlow(graph)
	flow := Block{Node: owner, Scope: scope, Graph: graph, Own: true, Stated: true}
	return append(append(locals, flow), results...)
}

func addCasePerformNodes(graph *ActionGraph, members []ast.Node, scope *symbols.Scope) {
	for _, member := range members {
		node, ok := unwrapMembership(member).(*ast.PerformActionNode)
		if !ok {
			continue
		}
		graph.Nodes = append(graph.Nodes, node)
		graph.Bodies[node] = []Statement{performEffect(node, scope)}
	}
}

func caseOutputBinding(node ast.Node) bool {
	usage, ok := node.(*ast.Usage)
	if !ok || usage.Kind != ast.UsageAttribute || usage.Value == nil {
		return false
	}
	for _, relationship := range usage.Relationships {
		if relationship != nil && relationship.Kind == ast.RelRedefines {
			return true
		}
	}
	return false
}

func caseTrailingResults(body []ast.Node, scope *symbols.Scope, trailing map[ast.Node]bool) map[ast.Node]bool {
	for i := len(body) - 1; i >= 0; i-- {
		member := body[i]
		if trailing[member] || statesNoStep(unwrapMembership(member)) {
			continue
		}
		if isFlowNode(unwrapMembership(member)) {
			break
		}
		stmt, states := calcStep(member, scope)
		if !states {
			continue
		}
		if declared, ok := stmt.(Declare); ok {
			usage, _ := unwrapMembership(member).(*ast.Usage)
			if declared.Value != nil && usage != nil && !usage.ValueIsInitial {
				trailing[member] = true
				continue
			}
		}
		if IsResult(stmt) {
			trailing[member] = true
			continue
		}
		break
	}
	return trailing
}

func addCaseStatementRuns(graph *ActionGraph, body []ast.Node, runs map[ast.Node]Statement) {
	if len(runs) == 0 {
		return
	}
	if graph.StatementRuns == nil {
		graph.StatementRuns = make(map[ast.Node]bool)
	}
	existing := make(map[ast.Node]bool, len(graph.Nodes))
	for _, node := range graph.Nodes {
		existing[node] = true
	}
	var ordered []ast.Node
	seen := make(map[ast.Node]bool, len(graph.Nodes)+len(runs))
	for _, member := range body {
		node := unwrapMembership(member)
		if stmt, ok := runs[node]; ok {
			graph.StatementRuns[node] = true
			graph.Bodies[node] = []Statement{stmt}
			ordered = append(ordered, node)
			seen[node] = true
		} else if existing[node] && !seen[node] {
			ordered = append(ordered, node)
			seen[node] = true
		}
	}
	for _, node := range graph.Nodes {
		if !seen[node] {
			ordered = append(ordered, node)
		}
	}
	graph.Nodes = ordered
}

func caseFlowConnector(member ast.Node) bool {
	usage, ok := member.(*ast.Usage)
	return ok && (usage.Kind == ast.UsageBinding || usage.Kind == ast.UsageFlow)
}

// isReturn reports a `return` of a body, a result parameter wherever it is declared.
func isReturn(stmt Statement) bool {
	_, ok := stmt.(Return)
	return ok
}

// trailingResults marks the members ending a body with results: the statements
// after its last step, each returning on some path through it (IsResult). Control
// flow returning among the steps stays a step, its effects in declared order.
func trailingResults(body []ast.Node, scope *symbols.Scope) map[ast.Node]bool {
	trailing := map[ast.Node]bool{}
	for i := len(body) - 1; i >= 0; i-- {
		member := body[i]
		if statesNoStep(member) {
			continue
		}
		if isFlowNode(member) {
			break
		}
		stmt, states := calcStep(member, scope)
		if !states {
			continue
		}
		if !IsResult(stmt) {
			break
		}
		trailing[member] = true
	}
	return trailing
}

// IsResult reports a statement that returns a value on some path through it: a
// `return`, or control flow with one inside. The results end a lowered body.
func IsResult(stmt Statement) bool {
	return Returns([]Statement{stmt})
}

// stepName names a step of a case body for a diagnostic.
func stepName(node ast.Node) string {
	if name := getNodeName(node); name != "" {
		return strconv.Quote(name)
	}
	return "an unnamed step"
}

// StartFlow gives a flow performed whole — a case body's, a state behavior's, an
// action's — the nodes its performance starts at: the ordered part's start, where
// no `first` states one its one unpreceded step, and every composite subaction no
// succession leads to as a concurrent start (startsConcurrently). Where nothing
// can start a flow stating steps, the graph keeps no start and running it
// reports why (FlowStartError).
func StartFlow(graph *ActionGraph) {
	if graph.UnstatedCaseFlow {
		graph.Initial = nil
		graph.Concurrent = append([]ast.Node(nil), graph.Nodes...)
		return
	}
	if graph.Initial == nil {
		if start, err := CaseFlowStart(graph); err == nil {
			graph.Initial = start
		}
	}
	graph.Concurrent = unorderedSubactions(graph)
}

// CaseFlowStart finds the step a flow starts at where no `first` or start node
// states one: the single step no succession leads to that the flow performs
// (performedStep). Two such steps leave the start unstated, and none is a
// cycle; either is reported.
func CaseFlowStart(graph *ActionGraph) (ast.Node, error) {
	preceded := precededNodes(graph)
	var starts []ast.Node
	for _, node := range graph.Nodes {
		if _, final := node.(*ast.FinalNode); !final && !preceded[node] && performedStep(graph, node) {
			starts = append(starts, node)
		}
	}
	switch len(starts) {
	case 0:
		return nil, fmt.Errorf("the successions form a cycle among the steps, leaving none to start at")
	case 1:
		return starts[0], nil
	default:
		return nil, fmt.Errorf("no succession leads to %s or to %s; 'first' names the step the flow starts at",
			stepName(starts[0]), stepName(starts[1]))
	}
}
