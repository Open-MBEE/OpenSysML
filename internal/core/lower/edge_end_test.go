package lower

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// An edge end bound to a member by position, rather than by a name, is named in
// the diagnostic by what the author wrote — never by a Go type.
func TestPositionalEdgeEndNamesTheNotation(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "a declaration that is no action node",
			src:  `action test { first start; then part { } done; }`,
			want: `anonymous part usage`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := parser.New(source.New("test.sysml", []byte(test.src)))
			root := p.ParseFile()
			usage := root.Members[0].(*ast.Membership).Member.(*ast.Usage)

			_, err := ToActionGraph(usage, nil)
			if err == nil {
				t.Fatalf("ToActionGraph succeeded, want an error naming the end")
			}
			if strings.Contains(err.Error(), "*ast.") {
				t.Fatalf("diagnostic names a Go type: %v", err)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("diagnostic %q does not name %s", err, test.want)
			}
		})
	}
}

// A performed action sequenced by a `then` is a node of the flow, named after
// the action it performs, so the succession reaches it by name.
func TestSequencedPerformIsANode(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			first start;
			action x;
			then perform doIt;
			then done;
			succession first start then x;
		}
	`)

	var perform ast.Node
	for _, node := range graph.Nodes {
		if getNodeName(node) == "doIt" {
			perform = node
		}
	}
	if perform == nil {
		t.Fatalf("the performed action is no node of the graph: %v", graph.Nodes)
	}
	x := nodeNamed(t, graph, "x")
	if got := graph.Edges[x]; len(got) != 1 || got[0].Target != perform {
		t.Fatalf("x has successors %v, want the performed action", got)
	}
	if len(graph.Edges[perform]) != 1 || graph.Edges[perform][0].Target != graph.Finals[0] {
		t.Fatalf("performed action has successors %v, want the final node", graph.Edges[perform])
	}
}

// statementKeyword names every member kind an edge can bind by position, so no
// diagnostic falls back to a Go type name.
func TestStatementKeywordNamesEveryPositionalMember(t *testing.T) {
	members := []ast.Node{
		&ast.WhileLoopActionNode{Kind: ast.LoopWhile},
		&ast.IfActionNode{},
		&ast.AssignmentActionNode{},
		&ast.SendStatement{},
		&ast.TerminateStatement{},
		&ast.PerformActionNode{},
		&ast.Usage{Kind: ast.UsageAction},
	}
	for _, member := range members {
		if got := statementKeyword(member); strings.Contains(got, "*ast.") || got == "" {
			t.Errorf("statementKeyword(%T) = %q", member, got)
		}
	}
}
