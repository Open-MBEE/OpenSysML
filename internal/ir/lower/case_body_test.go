package lower

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// caseBodyOf parses src and lowers the body of the first analysis def it declares.
func caseBodyOf(t *testing.T, src string) []Statement {
	t.Helper()
	p := parser.New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	for _, member := range root.Members {
		def, ok := unwrapMembership(member).(*ast.Definition)
		if ok && def.Kind == ast.DefAnalysisCase {
			return CalcBody(def, def.Members, nil)
		}
	}
	t.Fatal("no analysis def in the source")
	return nil
}

// kinds names the statements' kinds in order, for a comparison.
func kinds(stmts []Statement) []string {
	names := make([]string, len(stmts))
	for i, stmt := range stmts {
		switch stmt.(type) {
		case Block:
			names[i] = "flow"
		case If:
			names[i] = "if"
		case Return:
			names[i] = "return"
		case Declare:
			names[i] = "declare"
		default:
			names[i] = "other"
		}
	}
	return names
}

func wantKinds(t *testing.T, stmts []Statement, want ...string) {
	t.Helper()
	got := kinds(stmts)
	if len(got) != len(want) {
		t.Fatalf("lowered to %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("lowered to %v, want %v", got, want)
		}
	}
}

// A case body's `return`s are its results wherever declared; control flow that
// returns on some path is a result only where it ends the body, after its last step,
// so its other effects keep their place among the steps.
func TestCaseBodyEndsWithItsResults(t *testing.T) {
	t.Run("a branch ending the body is a result", func(t *testing.T) {
		body := caseBodyOf(t, `
			analysis def Ending {
				subject s : S;
				perform action p : P;
				if s.x > 1 {
					out r : Integer = s.x;
				}
				return r : Integer = 0;
			}
		`)
		wantKinds(t, body, "flow", "if", "return")
		if flow := body[0].(Block); len(flow.Graph.Nodes) != 1 {
			t.Errorf("the flow has %d node(s), want the performed action alone", len(flow.Graph.Nodes))
		}
	})

	t.Run("a branch among the steps stays a step", func(t *testing.T) {
		body := caseBodyOf(t, `
			analysis def Among {
				subject s : S;
				if s.ready {
					assign s.x := 1;
					out r : Integer = s.x;
				}
				perform action p : P;
				return r : Integer = s.x;
			}
		`)
		wantKinds(t, body, "flow", "return")
		flow := body[0].(Block)
		if len(flow.Graph.Nodes) != 2 {
			t.Fatalf("the flow has %d node(s), want the branch's run then the performed action", len(flow.Graph.Nodes))
		}
		if steps := flow.Graph.Bodies[flow.Graph.Nodes[0]]; len(steps) != 1 || kinds(steps)[0] != "if" {
			t.Errorf("the first node runs %v, want the branch", kinds(steps))
		}
	})

	t.Run("trailing results follow a local", func(t *testing.T) {
		body := caseBodyOf(t, `
			analysis def Trailing {
				subject s : S;
				perform action p : P;
				attribute k : Integer := s.x;
				if k > 1 {
					out r : Integer = k;
				}
				return r : Integer = 0;
			}
		`)
		wantKinds(t, body, "flow", "if", "return")
	})
}
