package passes

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// performStatementModel parses src, then rebuilds its tree with the action
// usage named `call` replaced by a `perform` statement of the usage's value.
func performStatementModel(t *testing.T, src string) *ast.RootNamespace {
	t.Helper()
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	replaced := false
	out := &ast.RootNamespace{NodeBase: root.NodeBase, Members: rebuildMembers(root.Members, &replaced)}
	if !replaced {
		t.Fatal("no action usage named call to replace")
	}
	return out
}

func rebuildMembers(members []ast.Node, replaced *bool) []ast.Node {
	out := make([]ast.Node, len(members))
	for i, m := range members {
		out[i] = rebuildMember(m, replaced)
	}
	return out
}

func rebuildMember(n ast.Node, replaced *bool) ast.Node {
	switch d := n.(type) {
	case *ast.Membership:
		m := *d
		m.Member = rebuildMember(d.Member, replaced)
		return &m
	case *ast.Package:
		p := *d
		p.Members = rebuildMembers(d.Members, replaced)
		return &p
	case *ast.Definition:
		def := *d
		def.Members = rebuildMembers(d.Members, replaced)
		return &def
	case *ast.Usage:
		if inv := d.PerformedInvocation(); inv != nil && d.Ident.Name == "call" {
			*replaced = true
			return &ast.PerformActionNode{NodeBase: d.NodeBase, ActionRef: inv}
		}
		return n
	default:
		return n
	}
}

const performStatementSrc = `
	package A {
		private import ScalarValues::*;
		%s def tag { in x : %s; %s }
	}
	package B {
		private import ScalarValues::*;
		%s def tag { in x : %s; %s }
	}
	package test {
		private import ScalarValues::*;
		private import A::*;
		private import B::*;
		action def Outer {
			action call = tag(%s);
		}
	}
`

// performDecl is one same-named declaration a `perform` statement may select.
type performDecl struct{ kind, paramType string }

func (d performDecl) body() string {
	if d.kind == "calc" {
		return `return : Integer = 1;`
	}
	return `out code : Integer;`
}

func performStatementDiags(t *testing.T, a, b performDecl, arg string) []diag.Diagnostic {
	t.Helper()
	src := fmt.Sprintf(performStatementSrc, a.kind, a.paramType, a.body(), b.kind, b.paramType, b.body(), arg)
	return libraryDiagsOf(performStatementModel(t, src))
}

// A `perform tag(x);` statement selects among actions as `action a = tag(x);`
// does: a same-named calc is not a candidate, the argument is checked against
// the action selected, and a name only calcs declare performs no action.
func TestPerformStatementSelectsAmongActions(t *testing.T) {
	calc := performDecl{"calc", "Integer"}
	if diags := performStatementDiags(t, calc, performDecl{"action", "Real"}, "3"); len(diags) != 0 {
		t.Fatalf("expected no diagnostics, got %v", diags)
	}
	diags := performStatementDiags(t, calc, performDecl{"action", "String"}, "3")
	if len(diags) != 1 || diags[0].Code != "type.expr" ||
		!strings.Contains(diags[0].Message, "argument 1 of tag expects String, found Natural") {
		t.Fatalf("expected the argument mismatch against B::tag, got %v", diags)
	}
	diags = performStatementDiags(t, calc, performDecl{"calc", "Real"}, "3")
	if len(diags) != 1 || diags[0].Code != "usage-reference-kind" || diags[0].Message != "Must reference an action." {
		t.Fatalf("expected calcs to be refused as an action, got %v", diags)
	}
}

// Two actions the argument fits equally well leave a `perform` statement ambiguous.
func TestPerformStatementAmbiguous(t *testing.T) {
	diags := performStatementDiags(t, performDecl{"action", "Integer"}, performDecl{"action", "Integer"}, "3")
	want := "call of tag is ambiguous between A::tag, B::tag"
	if len(diags) != 1 || diags[0].Code != "invocation-ambiguous" || diags[0].Message != want {
		t.Fatalf("expected one invocation-ambiguous diagnostic %q, got %v", want, diags)
	}
}
