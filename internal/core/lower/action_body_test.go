package lower

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// An action node's statements are lowered in declaration order, and its accept
// parameter is lowered with the type it was declared with, so the executor never
// has to walk the node's members again.
func TestActionBodyLowering(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			attribute total : Integer = 0;
			first start;
			action sender {
				send 5 to reader;
				assign total := 1;
				send "note" to logger;
			}
			action reader accept payload : Warning;
			done;
			succession first start then sender;
			succession first sender then reader;
			succession first reader then done;
		}
	`)

	sender := nodeNamed(t, graph, "sender")
	body := graph.Bodies[sender]
	if len(body) != 3 {
		t.Fatalf("sender lowered to %d statements, want 3: %#v", len(body), body)
	}

	first, ok := body[0].(Send)
	if !ok {
		t.Fatalf("statement 0 = %T, want Send", body[0])
	}
	if first.Target != "reader" {
		t.Errorf("statement 0 target = %q, want %q", first.Target, "reader")
	}
	if assign, ok := body[1].(Assign); !ok {
		t.Errorf("statement 1 = %T, want Assign", body[1])
	} else if assign.Target != "total" {
		t.Errorf("statement 1 target = %q, want %q", assign.Target, "total")
	}
	if third, ok := body[2].(Send); !ok {
		t.Errorf("statement 2 = %T, want Send", body[2])
	} else if third.Target != "logger" {
		t.Errorf("statement 2 target = %q, want %q", third.Target, "logger")
	}

	reader := nodeNamed(t, graph, "reader")
	accept, ok := graph.Accepts[reader]
	if !ok {
		t.Fatal("reader lowered without an accept")
	}
	if accept.ParamName != "payload" || ast.QualifiedText(accept.SignalType) != "Warning" {
		t.Errorf("reader accept = %+v, want {payload Warning}", accept)
	}

	if _, ok := graph.Accepts[nodeNamed(t, graph, "sender")]; ok {
		t.Error("sender lowered with an accept it does not declare")
	}
}

// An accept typed with a qualified name keeps the whole qualifier, so the
// runtime resolves the exact definition rather than any same-named one.
func TestAcceptKeepsItsQualifiedSignalType(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			first start;
			action reader accept payload : Alerts::Warning;
			done;
			succession first start then reader;
			succession first reader then done;
		}
	`)

	accept, ok := graph.Accepts[nodeNamed(t, graph, "reader")]
	if !ok {
		t.Fatal("reader lowered without an accept")
	}
	if got := ast.QualifiedText(accept.SignalType); got != "Alerts::Warning" {
		t.Errorf("accept signal type = %q, want %q", got, "Alerts::Warning")
	}
}

// A globally qualified accept type keeps its root marker distinct from a
// package literally named `$`, since the two resolve to different definitions.
func TestAcceptKeepsTheGlobalQualifier(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			first start;
			action reader accept payload : $::Alerts::Warning;
			action other accept note : '$'::Warning;
			done;
			succession first start then reader;
			succession first reader then other;
			succession first other then done;
		}
	`)

	accept, ok := graph.Accepts[nodeNamed(t, graph, "reader")]
	if !ok {
		t.Fatal("reader lowered without an accept")
	}
	if !accept.SignalType.Global || ast.QualifiedText(accept.SignalType) != "$::Alerts::Warning" {
		t.Errorf("accept signal type = %+v, want global Alerts::Warning", accept.SignalType)
	}

	other, ok := graph.Accepts[nodeNamed(t, graph, "other")]
	if !ok {
		t.Fatal("other lowered without an accept")
	}
	if other.SignalType.Global || other.SignalType.Parts[0].Text != "$" {
		t.Errorf("accept signal type = %+v, want package '$'", other.SignalType)
	}
}

// A loop and a conditional are lowered as body statements of the node they were
// written in, with the block each one owns lowered in turn, so the executor
// reads control flow from the graph instead of walking the AST again.
func TestActionBodyLoopAndConditionalLowering(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			attribute total : Integer = 0;
			attribute steps = (1, 2);
			first start;
			action driver {
				while total < 5 {
					attribute bump : Integer = 1;
					assign total := total + bump;
					if total == 2 {
						send total to logger;
					} else {
						assign total := total + 1;
					}
				}
				loop {
					assign total := total + 1;
				} until total > 9;
				for s in steps {
					assign total := total + s;
				}
			}
			done;
			succession first start then driver;
			succession first driver then done;
		}
	`)

	body := graph.Bodies[nodeNamed(t, graph, "driver")]
	if len(body) != 3 {
		t.Fatalf("driver lowered to %d statements, want 3: %#v", len(body), body)
	}

	while, ok := body[0].(Loop)
	if !ok {
		t.Fatalf("statement 0 = %T, want Loop", body[0])
	}
	if while.Kind != ast.LoopWhile {
		t.Errorf("statement 0 kind = %v, want %v", while.Kind, ast.LoopWhile)
	}
	if while.Condition == nil {
		t.Error("a while loop lowered without its condition")
	}
	if len(while.Body.Statements) != 3 {
		t.Fatalf("while body lowered to %d statements, want 3: %#v", len(while.Body.Statements), while.Body.Statements)
	}
	if declare, ok := while.Body.Statements[0].(Declare); !ok {
		t.Errorf("while body statement 0 = %T, want Declare", while.Body.Statements[0])
	} else if declare.Name != "bump" || declare.Value == nil {
		t.Errorf("while body statement 0 = %+v, want bump with a value", declare)
	}
	nested, ok := while.Body.Statements[2].(If)
	if !ok {
		t.Fatalf("while body statement 2 = %T, want If", while.Body.Statements[2])
	}
	if nested.Condition == nil {
		t.Error("a conditional lowered without its condition")
	}
	if len(nested.Then.Statements) != 1 {
		t.Fatalf("then branch lowered to %d statements, want 1", len(nested.Then.Statements))
	}
	if send, ok := nested.Then.Statements[0].(Send); !ok {
		t.Errorf("then branch statement = %T, want Send", nested.Then.Statements[0])
	} else if send.Target != "logger" {
		t.Errorf("then branch send target = %q, want %q", send.Target, "logger")
	}
	if nested.Else == nil {
		t.Fatal("a conditional with an else branch lowered without one")
	}
	if len(nested.Else.Statements) != 1 {
		t.Errorf("else branch lowered to %d statements, want 1", len(nested.Else.Statements))
	}

	until, ok := body[1].(Loop)
	if !ok {
		t.Fatalf("statement 1 = %T, want Loop", body[1])
	}
	if until.Kind != ast.LoopUntil {
		t.Errorf("statement 1 kind = %v, want %v", until.Kind, ast.LoopUntil)
	}
	if until.Condition == nil {
		t.Error("a post-condition loop lowered without its until condition")
	}

	forLoop, ok := body[2].(Loop)
	if !ok {
		t.Fatalf("statement 2 = %T, want Loop", body[2])
	}
	if forLoop.Kind != ast.LoopFor {
		t.Errorf("statement 2 kind = %v, want %v", forLoop.Kind, ast.LoopFor)
	}
	if forLoop.Variable != "s" {
		t.Errorf("for loop variable = %q, want %q", forLoop.Variable, "s")
	}
	if forLoop.Collection == nil {
		t.Error("a for loop lowered without its collection")
	}
	if forLoop.Condition != nil {
		t.Error("a for loop lowered with a condition it does not have")
	}
}

// A body member that is neither an executable statement nor a node of the
// block's own flow is lowered to Unsupported rather than dropped, so reaching it
// reports a diagnostic.
func TestActionBodyUnexecutableMemberIsLowered(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			attribute total : Integer = 0;
			first start;
			action driver {
				while total < 5 {
					part inner;
				}
			}
			done;
			succession first start then driver;
			succession first driver then done;
		}
	`)

	body := graph.Bodies[nodeNamed(t, graph, "driver")]
	loop, ok := body[0].(Loop)
	if !ok {
		t.Fatalf("statement 0 = %T, want Loop", body[0])
	}
	if len(loop.Body.Statements) != 1 {
		t.Fatalf("loop body lowered to %d statements, want 1", len(loop.Body.Statements))
	}
	unsupported, ok := loop.Body.Statements[0].(Unsupported)
	if !ok {
		t.Fatalf("loop body statement = %T, want Unsupported", loop.Body.Statements[0])
	}
	if !strings.Contains(unsupported.Description, "inner") {
		t.Errorf("description = %q, want it to name the member", unsupported.Description)
	}
}

// An accept node written in a loop body would have to suspend the action, which
// the block's flow has no token to park, so it is lowered as unsupported rather
// than passed over silently.
func TestAcceptInALoopBodyIsLoweredAsUnsupported(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			first start;
			action driver {
				loop {
					accept n : Integer;
				}
			}
			done;
			succession first start then driver;
			succession first driver then done;
		}
	`)

	body := graph.Bodies[nodeNamed(t, graph, "driver")]
	loop, ok := body[0].(Loop)
	if !ok {
		t.Fatalf("statement 0 = %T, want Loop", body[0])
	}
	flow := loop.Body.Graph
	if flow == nil {
		t.Fatalf("the loop body lowered to statements, want the flow its accept node states")
	}
	stmts := flow.Bodies[flow.Initial]
	if len(stmts) != 1 {
		t.Fatalf("the accept node lowered to %d statements, want 1", len(stmts))
	}
	unsupported, ok := stmts[0].(Unsupported)
	if !ok {
		t.Fatalf("the accept node lowered to %T, want Unsupported", stmts[0])
	}
	if !strings.Contains(unsupported.Description, "'accept'") {
		t.Errorf("description = %q, want it to name the accept", unsupported.Description)
	}
}

// A loop body written as an action body parameter is the body itself, whether or
// not it was named and whether or not it declares members, so it lowers to the
// block whose owner scopes the members — not to an unexecutable nested node.
func TestActionBodyParameterLowersToItsBlock(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			attribute total : Integer = 0;
			first start;
			action driver {
				loop action charging {
					assign total := total + 1;
				} until total > 2;
				loop action { } until true;
				for s in (1, 2) action stepping {
					assign total := total + s;
				}
			}
			done;
			succession first start then driver;
			succession first driver then done;
		}
	`)

	body := graph.Bodies[nodeNamed(t, graph, "driver")]
	if len(body) != 3 {
		t.Fatalf("driver lowered to %d statements, want 3: %#v", len(body), body)
	}

	named, ok := body[0].(Loop)
	if !ok {
		t.Fatalf("statement 0 = %T, want Loop", body[0])
	}
	charging := blockOwnedBy(t, named.Body, "charging")
	if len(charging.Statements) != 1 {
		t.Fatalf("named body lowered to %d statements, want 1: %#v", len(charging.Statements), charging.Statements)
	}
	if _, ok := charging.Statements[0].(Assign); !ok {
		t.Errorf("named body statement = %T, want Assign", charging.Statements[0])
	}

	empty, ok := body[1].(Loop)
	if !ok {
		t.Fatalf("statement 1 = %T, want Loop", body[1])
	}
	if statements := blockOwnedBy(t, empty.Body, "").Statements; len(statements) != 0 {
		t.Errorf("empty body lowered to %d statements, want 0: %#v", len(statements), statements)
	}

	forLoop, ok := body[2].(Loop)
	if !ok {
		t.Fatalf("statement 2 = %T, want Loop", body[2])
	}
	if statements := blockOwnedBy(t, forLoop.Body, "stepping").Statements; len(statements) != 1 {
		t.Errorf("for body lowered to %d statements, want 1: %#v", len(statements), statements)
	}
}

// blockOwnedBy returns the block a loop's single body-parameter statement lowered
// to, checking the usage that owns it is the one named.
func blockOwnedBy(t *testing.T, body Block, name string) Block {
	t.Helper()
	if len(body.Statements) != 1 {
		t.Fatalf("loop body lowered to %d statements, want 1: %#v", len(body.Statements), body.Statements)
	}
	block, ok := body.Statements[0].(Block)
	if !ok {
		t.Fatalf("loop body statement = %T, want Block", body.Statements[0])
	}
	usage, ok := block.Node.(*ast.Usage)
	if !ok {
		t.Fatalf("block owner = %T, want *ast.Usage", block.Node)
	}
	if usage.Ident.Name != name {
		t.Errorf("block owner = %q, want %q", usage.Ident.Name, name)
	}
	return block
}

func actionGraphFor(t *testing.T, src string) *ActionGraph {
	t.Helper()
	graph, err := ToActionGraph(actionUsageOf(t, src), nil)
	if err != nil {
		t.Fatalf("ToActionGraph: %v", err)
	}
	return graph
}

// actionUsageOf parses src and returns the first action usage it declares.
func actionUsageOf(t *testing.T, src string) *ast.Usage {
	t.Helper()
	p := parser.New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}

	for _, member := range root.Members {
		membership, ok := member.(*ast.Membership)
		if !ok {
			continue
		}
		usage, ok := membership.Member.(*ast.Usage)
		if !ok || usage.Kind != ast.UsageAction {
			continue
		}
		return usage
	}
	t.Fatal("no action usage found")
	return nil
}

// An action's interface lowers whatever its body states, as the attributes alone and no
// flow, for a performance an external tool makes in the body's place.
func TestActionInterfaceLowering_IgnoresTheBody(t *testing.T) {
	usage := actionUsageOf(t, `
		action test {
			in x : Integer;
			out y : Integer;
			assign y := x;
		}
	`)
	if _, err := ToActionGraph(usage, nil); !errors.Is(err, ErrStatementOutsideFlow) {
		t.Fatalf("ToActionGraph error = %v, want %v", err, ErrStatementOutsideFlow)
	}
	graph, err := ToActionInterface(usage, nil)
	if err != nil {
		t.Fatalf("ToActionInterface: %v", err)
	}
	if len(graph.Attributes) != 2 || graph.Attributes[0].Name != "x" || graph.Attributes[1].Name != "y" {
		t.Errorf("attributes = %#v, want x and y", graph.Attributes)
	}
	if len(graph.Nodes) != 0 || graph.Initial != nil || len(graph.Bodies) != 0 {
		t.Errorf("interface carries a flow: nodes %v, initial %v, bodies %v", graph.Nodes, graph.Initial, graph.Bodies)
	}
}

func nodeNamed(t *testing.T, graph *ActionGraph, name string) ast.Node {
	t.Helper()
	for _, node := range graph.Nodes {
		if usage, ok := node.(*ast.Usage); ok && usage.Ident.Name == name {
			return node
		}
	}
	t.Fatalf("node %s not found in graph", name)
	return nil
}

// A valueless action attribute, including an output parameter, still reaches the
// graph as a feature the action owns.
func TestActionAttributeLowering_KeepsValuelessAttributes(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			attribute scratch : Integer = 0;
			out attribute n : Integer;
			first start;
			action start;
		}
	`)

	if len(graph.Attributes) != 2 {
		t.Fatalf("attributes = %#v, want scratch and n", graph.Attributes)
	}
	if got := graph.Attributes[0]; got.Name != "scratch" || got.Value == nil {
		t.Errorf("attribute 0 = %#v, want valued scratch", got)
	}
	if got := graph.Attributes[1]; got.Name != "n" || got.Value != nil {
		t.Errorf("attribute 1 = %#v, want valueless n", got)
	}
}

// actionGraphErr lowers the first action usage of src, reporting the error
// lowering returns rather than failing on it.
func actionGraphErr(t *testing.T, src string) (*ActionGraph, error) {
	t.Helper()
	p := parser.New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	for _, member := range root.Members {
		membership, ok := member.(*ast.Membership)
		if !ok {
			continue
		}
		usage, ok := membership.Member.(*ast.Usage)
		if !ok || usage.Kind != ast.UsageAction {
			continue
		}
		return ToActionGraph(usage, nil)
	}
	t.Fatal("no action usage found")
	return nil, nil
}
