package lower

import (
	"fmt"
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
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

// caseSteps lowers a body whose steps are action nodes: the locals it declares, one
// Block over the flow the steps state, then its results. A body stating successions
// or control nodes is the token flow an action body is (ToActionGraph); one stating
// none runs its steps in declaration order.
func caseSteps(owner ast.Node, body []ast.Node, scope *symbols.Scope) []Statement {
	if !statesOwnFlow(body) {
		var results []Statement
		graph := lowerBlockFlowWith(body, scope, func(graph *ActionGraph, nodes []ast.Node, member ast.Node) (Statement, bool) {
			if usage, ok := member.(*ast.Usage); ok {
				if stmt, connects := lowerBlockConnector(graph, nodes, usage, scope); connects {
					return stmt, stmt != nil
				}
			}
			stmt, states := calcStep(member, scope)
			if !states {
				return nil, false
			}
			if _, isResult := stmt.(Return); isResult {
				results = append(results, stmt)
				return nil, false
			}
			return stmt, true
		})
		flow := Block{Node: owner, Scope: scope, Graph: graph, Own: true}
		return append([]Statement{flow}, results...)
	}

	// The flow's nodes and the members sequencing them are the graph's; the
	// other members are the case's locals and results, as in a calc body.
	var locals, results []Statement
	sequenced := sequencedMembers(body)
	for _, member := range body {
		if isFlowNode(member) || outsideBlockFlow(member) || sequenced[member] {
			continue
		}
		stmt, states := calcStep(member, scope)
		if !states {
			continue
		}
		if _, isResult := stmt.(Return); isResult {
			results = append(results, stmt)
			continue
		}
		locals = append(locals, stmt)
	}
	graph, err := ToActionGraph(owner, scope)
	if err != nil {
		unsupported := Unsupported{
			Description: "the flow the steps of the body state: " + err.Error(),
			Node:        owner,
			Scope:       scope,
		}
		return append(append(locals, unsupported), results...)
	}
	// A flow no `first` starts begins at its one unpreceded step; where none is
	// found the graph keeps no start, and running it reports why.
	if graph.Initial == nil {
		if start, err := CaseFlowStart(graph); err == nil {
			graph.Initial = start
		}
	}
	flow := Block{Node: owner, Scope: scope, Graph: graph, Own: true, Stated: true}
	return append(append(locals, flow), results...)
}

// stepName names a step of a case body for a diagnostic.
func stepName(node ast.Node) string {
	if name := getNodeName(node); name != "" {
		return strconv.Quote(name)
	}
	return "an unnamed step"
}

// CaseFlowStart finds the step a case body's flow starts at where no `first` or
// start node states one: the single step no succession leads to. Two such steps
// leave the start unstated, and none is a cycle; either is reported.
func CaseFlowStart(graph *ActionGraph) (ast.Node, error) {
	preceded := make(map[ast.Node]bool, len(graph.Nodes))
	for _, edges := range graph.Edges {
		for _, edge := range edges {
			preceded[edge.Target] = true
		}
	}
	var starts []ast.Node
	for _, node := range graph.Nodes {
		if _, final := node.(*ast.FinalNode); !final && !preceded[node] {
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
