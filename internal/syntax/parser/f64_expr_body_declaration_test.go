package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// F64: a body expression is a calculation body (SysML.xtext ExpressionBody), so
// it may declare features of its own between its parameters and its result.
func TestF64BodyExprDeclaration(t *testing.T) {
	bodyOf := func(t *testing.T, src string) *ast.BodyExpr {
		t.Helper()
		sf := source.New("f64.sysml", []byte(src))
		p := New(sf)
		root := p.ParseFile()
		if len(p.Diagnostics) > 0 {
			t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
		}
		pkg := root.Members[0].(*ast.Membership).Member.(*ast.Package)
		def := pkg.Members[len(pkg.Members)-1].(*ast.Membership).Member.(*ast.Definition)
		last := def.Members[len(def.Members)-1]
		if m, ok := last.(*ast.Membership); ok {
			last = m.Member
		}
		var body ast.Node
		switch e := last.(type) {
		case *ast.InvocationExpr:
			if len(e.Args) == 1 {
				body = e.Args[0]
			}
		case *ast.SelectExpr:
			body = e.Body
		case *ast.CollectExpr:
			body = e.Body
		}
		b, ok := body.(*ast.BodyExpr)
		if !ok {
			t.Fatalf("body of %T = %T, want *ast.BodyExpr", last, body)
		}
		return b
	}

	t.Run("private_attribute_is_a_member", func(t *testing.T) {
		b := bodyOf(t, `package B {
			private import ScalarValues::*;
			private import ControlFunctions::forAll;
			calc def C {
				in radius : Real;
				(1..3)->forAll { in i : Real; private attribute scaled : Real = radius * i; scaled > 0.0 }
			}
		}`)
		if len(b.Params) != 1 || b.Params[0].Name != "i" {
			t.Fatalf("params = %v, want one named i", b.Params)
		}
		if len(b.Members) != 1 {
			t.Fatalf("members = %d, want the private attribute", len(b.Members))
		}
		if b.Result == nil {
			t.Error("result = nil, want the condition")
		}
	})

	t.Run("kind_keyword_naming_a_call_stays_the_result", func(t *testing.T) {
		b := bodyOf(t, `package B {
			private import ScalarValues::*;
			calc def objective { in x : Real; return : Real = x; }
			calc def C { in xs : Real[*]; xs->select {in ref a { doc /* pick */ } objective(a)} }
		}`)
		if len(b.Members) != 0 {
			t.Errorf("members = %d, want none: `objective(a)` is the result", len(b.Members))
		}
		if _, ok := b.Result.(*ast.InvocationExpr); !ok {
			t.Errorf("result = %T, want *ast.InvocationExpr", b.Result)
		}
	})
}

// A malformed declaration in a body expression must produce diagnostics, never
// a panic.
func TestF64BodyExprDeclarationNegative(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"visibility_only", "calc def C { xs->select { in i : Real; private } }"},
		{"declaration_no_terminator", "calc def C { xs->select { in i : Real; private attribute k : Real i > 0 } }"},
		{"declaration_no_value", "calc def C { xs->select { in i : Real; private attribute k : Real = ; i > 0 } }"},
		{"declaration_at_eof", "calc def C { xs->select { in i : Real; private attribute"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := source.New("f64_neg.sysml", []byte(tt.input))
			p := New(sf)
			if p.ParseFile() == nil {
				t.Fatal("ParseFile returned nil")
			}
			if len(p.Diagnostics) == 0 {
				t.Errorf("expected diagnostics for %q, got none", tt.input)
			}
		})
	}
}
