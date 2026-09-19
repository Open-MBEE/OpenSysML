package lower

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A block declaring an action node is lowered to a flow of its own: the runs of
// statements and the action nodes between them are its nodes, in declaration
// order, each succeeded by the next.
func TestBlockDeclaringAnActionNodeIsLoweredToAFlow(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			attribute total : Integer = 0;
			first start;
			action accumulate {
				while total < 3 {
					assign total := total + 1;
					assign total := total * 2;
					action bump {
						assign total := total + 1;
					}
					perform other;
					assign total := total - 1;
				}
			}
			done;
			succession first start then accumulate;
			succession first accumulate then done;
		}
	`)

	block := loopBodyOf(t, graph, "accumulate")
	if block.Graph == nil {
		t.Fatalf("loop body lowered to statements, want a flow of its own: %#v", block)
	}
	if len(block.Statements) != 0 {
		t.Errorf("loop body kept %d statements beside its flow", len(block.Statements))
	}

	flow := block.Graph
	if len(flow.Nodes) != 4 {
		t.Fatalf("block flow has %d nodes, want 4: %#v", len(flow.Nodes), flow.Nodes)
	}
	if flow.Initial != flow.Nodes[0] {
		t.Errorf("block flow starts at %T, want its first node %T", flow.Initial, flow.Nodes[0])
	}
	for i := 0; i+1 < len(flow.Nodes); i++ {
		successors := flow.Edges[flow.Nodes[i]]
		if len(successors) != 1 || successors[0].Target != flow.Nodes[i+1] {
			t.Errorf("node %d succeeds to %v, want the node written after it", i, successors)
		}
	}
	if last := flow.Edges[flow.Nodes[3]]; len(last) != 0 {
		t.Errorf("the last node succeeds to %v, want nothing", last)
	}

	// The two assignments written together are one step of the flow, so what the
	// first of them declares is in scope for the second.
	if !flow.StatementRuns[flow.Nodes[0]] {
		t.Errorf("the first node is not a run of statements: %#v", flow.Nodes[0])
	}
	if got := len(flow.Bodies[flow.Nodes[0]]); got != 2 {
		t.Errorf("the first node runs %d statements, want the 2 written together", got)
	}

	// The nested action declaration is a node performed in a frame of its own,
	// whose body holds its statements.
	if body := flow.Bodies[flow.Nodes[1]]; len(body) != 1 {
		t.Errorf("nested action holds %d statements, want 1: %#v", len(body), body)
	} else if _, ok := body[0].(Assign); !ok {
		t.Errorf("nested action's statement lowered to %#v, want its assignment", body[0])
	}
	if flow.StatementRuns[flow.Nodes[1]] {
		t.Errorf("the nested action node is recorded as a run of statements")
	}

	// The `perform` is a node of the flow too, performed when the token reaches
	// it: a usage with no body of its own, naming the action it performs.
	if performed, ok := flow.Nodes[2].(*ast.Usage); !ok || performed.Keyword != "perform" {
		t.Errorf("perform lowered to %#v, want the perform usage as a node", flow.Nodes[2])
	}
	if body := flow.Bodies[flow.Nodes[2]]; len(body) != 0 {
		t.Errorf("perform holds %d statements, want none: %#v", len(body), body)
	}

	// The action nodes the loop body declares are recorded as those of the node
	// running the loop, so a read of `bump.x` finds the node by name.
	declared := graph.BlockNodes[nodeNamed(t, graph, "accumulate")]
	if len(declared) != 2 || declared[0] != flow.Nodes[1] || declared[1] != flow.Nodes[2] {
		t.Errorf("accumulate declares block nodes %#v, want the nested action and the perform", declared)
	}
}

// A nested action declared in a block declares a flow of its own in turn, so a
// block's flow nests as deeply as the model writes it.
func TestBlockFlowNestsTwoLevels(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			attribute total : Integer = 0;
			first start;
			action accumulate {
				while total < 3 {
					action outer {
						assign total := total + 1;
						action inner {
							assign total := total * 2;
						}
					}
				}
			}
			done;
			succession first start then accumulate;
			succession first accumulate then done;
		}
	`)

	block := loopBodyOf(t, graph, "accumulate")
	if block.Graph == nil {
		t.Fatalf("loop body lowered to statements, want a flow of its own: %#v", block)
	}
	outerNode := block.Graph.Nodes[0]
	outer, ok := block.Graph.Bodies[outerNode][0].(Block)
	if !ok {
		t.Fatalf("outer action lowered to %#v, want a block", block.Graph.Bodies[outerNode][0])
	}
	if outer.Graph == nil {
		t.Fatalf("outer action lowered to statements, want a flow of its own: %#v", outer)
	}
	if len(outer.Graph.Nodes) != 2 {
		t.Fatalf("outer flow has %d nodes, want the assignment and the nested action", len(outer.Graph.Nodes))
	}
	innerNode := outer.Graph.Nodes[1]
	if body := outer.Graph.Bodies[innerNode]; len(body) != 1 {
		t.Errorf("inner action holds %d statements, want 1: %#v", len(body), body)
	} else if _, ok := body[0].(Assign); !ok {
		t.Errorf("inner action's statement lowered to %#v, want its assignment", body[0])
	}

	// Each level records the nodes its blocks declare: the loop's body declares
	// outer, and outer's own flow declares inner.
	if declared := graph.BlockNodes[nodeNamed(t, graph, "accumulate")]; len(declared) != 1 || declared[0] != outerNode {
		t.Errorf("accumulate declares block nodes %#v, want outer alone", declared)
	}
	if declared := block.Graph.BlockNodes[outerNode]; len(declared) != 1 || declared[0] != innerNode {
		t.Errorf("outer declares block nodes %#v, want inner alone", declared)
	}
}

// A block stating an edge of its own is not a flow of its own: the flow it states
// is an action body's, so its members keep their statement form and the ones no
// statement executes are reported when reached.
func TestBlockStatingItsOwnEdgeKeepsItsStatements(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			attribute total : Integer = 0;
			first start;
			action accumulate {
				while total < 3 {
					action bump {
						assign total := total + 1;
					}
					succession first bump then bump;
				}
			}
			done;
			succession first start then accumulate;
			succession first accumulate then done;
		}
	`)

	block := loopBodyOf(t, graph, "accumulate")
	if block.Graph != nil {
		t.Fatalf("loop body lowered to a flow of its own, want statements: %#v", block.Graph.Nodes)
	}
	if len(block.Statements) == 0 {
		t.Fatal("loop body lowered to no statements")
	}
	if _, ok := block.Statements[0].(Unsupported); !ok {
		t.Errorf("nested action lowered to %#v, want Unsupported", block.Statements[0])
	}
}

// A nested action's parameters are features of its performance, recorded with
// the values their declarations give them, and no statement of its body.
func TestNestedActionParametersAreLowered(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			attribute total : Integer = 0;
			first start;
			action accumulate {
				for i in 1..3 {
					action bump {
						in stride = 2;
						out reached : Integer;
						assign total := total + stride;
						out result = total;
					}
				}
			}
			done;
			succession first start then accumulate;
			succession first accumulate then done;
		}
	`)

	block := loopBodyOf(t, graph, "accumulate")
	if block.Graph == nil {
		t.Fatalf("loop body lowered to statements, want a flow of its own: %#v", block)
	}
	nested := block.Graph.Nodes[0]
	body := block.Graph.Bodies[nested]
	if len(body) != 1 {
		t.Fatalf("nested action holds %d statements, want its one assignment: %#v", len(body), body)
	}
	if assigned, ok := body[0].(Assign); !ok || assigned.Target != "total" {
		t.Errorf("nested action's statement lowered to %#v, want the assignment to total", body[0])
	}
	features := block.Graph.Features[nested]
	if len(features) != 3 {
		t.Fatalf("nested action declares %d features, want stride, reached and result: %#v", len(features), features)
	}
	for i, want := range []struct {
		name      string
		direction ast.FeatureDirection
		valued    bool
	}{{"stride", ast.DirIn, true}, {"reached", ast.DirOut, false}, {"result", ast.DirOut, true}} {
		got := features[i]
		if got.Name != want.name || got.Direction != want.direction || (got.Value != nil) != want.valued {
			t.Errorf("feature %d lowered to %#v, want %s %s valued=%v", i, got, want.direction, want.name, want.valued)
		}
	}
}

func TestBlockBindingsAndFlowsAtItsNodesPinsAreLoweredIntoItsFlow(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			attribute total : Integer = 0;
			first start;
			action accumulate {
				for i in 1..3 {
					bind dbl.a = i;
					action dbl { in a : Integer; out sum : Integer; assign sum := a + a; }
					flow dbl.sum to acc.n;
					action acc { in n : Integer; assign total := total + n; }
					bind acc = total;
				}
			}
			done;
			succession first start then accumulate;
			succession first accumulate then done;
		}
	`)

	block := loopBodyOf(t, graph, "accumulate")
	if block.Graph == nil {
		t.Fatalf("loop body lowered to statements, want a flow of its own: %#v", block)
	}
	flow := block.Graph
	var actions []ast.Node
	for _, node := range flow.Nodes {
		if !flow.StatementRuns[node] {
			actions = append(actions, node)
		}
	}
	if len(actions) != 2 {
		t.Fatalf("block flow holds %d action nodes, want dbl and acc: %#v", len(actions), actions)
	}
	dbl, acc := actions[0], actions[1]
	if len(flow.Bindings) != 1 || flow.Bindings[0].Node != dbl || flow.Bindings[0].Pin != "a" {
		t.Errorf("block flow lowered bindings %#v, want the one at dbl.a", flow.Bindings)
	}
	flows := flow.DataFlows[dbl]
	if len(flows) != 1 || flows[0].SourcePin != "sum" || flows[0].Target != acc || flows[0].TargetPin != "n" {
		t.Errorf("block flow lowered object flows %#v, want dbl.sum to acc.n", flows)
	}
	var unsupported []Unsupported
	for _, node := range flow.Nodes {
		if !flow.StatementRuns[node] {
			continue
		}
		for _, stmt := range flow.Bodies[node] {
			if u, ok := stmt.(Unsupported); ok {
				unsupported = append(unsupported, u)
			}
		}
	}
	if len(unsupported) != 1 || !strings.Contains(unsupported[0].Description, "names an action node but no pin of it") {
		t.Errorf("block flow reports %#v, want the binding at acc itself as unsupported", unsupported)
	}
}

// A block-declared action stating a flow of its own (`first`, a succession) owns
// that flow as a subflow of the block's graph, as a node of the action's flow
// would, rather than lowering its flow constructs to unsupported statements.
func TestBlockNodeStatingItsOwnFlowOwnsASubflow(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			attribute total : Integer = 0;
			first start;
			action accumulate {
				for i in 1..3 {
					action step {
						out v : Integer;
						first start;
						then action one { out w : Integer; assign w := i; }
						succession one then two;
						action two { assign v := one.w + 1; }
						then done;
					}
					assign total := total + step.v;
				}
			}
			done;
			succession first start then accumulate;
			succession first accumulate then done;
		}
	`)

	flow := loopBodyOf(t, graph, "accumulate").Graph
	if flow == nil {
		t.Fatal("loop body lowered to statements, want a flow of its own")
	}
	step := nodeNamed(t, flow, "step")
	sub, owns := flow.Subflows[step]
	if !owns || sub == nil {
		t.Fatalf("step owns no subflow: %#v", flow.Subflows)
	}
	if sub.Err != nil {
		t.Fatalf("step's own flow failed to lower: %v", sub.Err)
	}
	if len(flow.Bodies[step]) != 0 {
		t.Errorf("step lowered statements %#v beside its own flow", flow.Bodies[step])
	}
	if len(flow.Features[step]) != 1 || flow.Features[step][0].Name != "v" {
		t.Errorf("step declares features %#v, want its pin v", flow.Features[step])
	}
	if sub.Graph.Initial == nil {
		t.Fatal("step's own flow has no initial node")
	}
	one, two := nodeNamed(t, sub.Graph, "one"), nodeNamed(t, sub.Graph, "two")
	if next := sub.Graph.Edges[one]; len(next) != 1 || next[0].Target != two {
		t.Errorf("one succeeds to %v, want two", next)
	}
	if len(sub.Graph.Features[one]) != 1 || sub.Graph.Features[one][0].Name != "w" {
		t.Errorf("one declares features %#v, want its pin w", sub.Graph.Features[one])
	}
	if next := sub.Graph.Edges[two]; len(next) != 1 {
		t.Errorf("two succeeds to %v, want done", next)
	} else if _, done := next[0].Target.(*ast.FinalNode); !done {
		t.Errorf("two succeeds to %T, want the final node", next[0].Target)
	}
}

// loopBodyOf returns the block the named action node's loop runs.
func loopBodyOf(t *testing.T, graph *ActionGraph, node string) Block {
	t.Helper()
	body := graph.Bodies[nodeNamed(t, graph, node)]
	if len(body) != 1 {
		t.Fatalf("action node %s lowered to %d statements, want the loop", node, len(body))
	}
	loop, ok := body[0].(Loop)
	if !ok {
		t.Fatalf("action node %s lowered to %#v, want a loop", node, body[0])
	}
	return loop.Body
}
