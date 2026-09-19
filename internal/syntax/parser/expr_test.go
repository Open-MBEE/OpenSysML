package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestParseExprParam(t *testing.T) {
	input := `calc def 'if' {
		in expr thenValue[0..1] { return : Anything[0..*] ordered nonunique; }
	}`

	sf := source.New("test.sysml", []byte(input))
	p := New(sf)
	ns := p.ParseFile()

	if len(p.Diagnostics) > 0 {
		for _, d := range p.Diagnostics {
			t.Logf("Diagnostic: %s (offset=%d)", d.Message, d.Span.Offset)
		}
	}

	if len(ns.Members) != 1 {
		t.Fatalf("Expected 1 member, got %d", len(ns.Members))
	}

	m1, ok := ns.Members[0].(*ast.Membership)
	if !ok {
		t.Fatalf("Expected Membership, got %T", ns.Members[0])
	}

	def, ok := m1.Member.(*ast.Definition)
	if !ok {
		t.Fatalf("Expected Definition, got %T", m1.Member)
	}

	dump := ast.Dump(def)
	t.Logf("AST dump:\n%s", dump)

	// Should have 1 member: thenValue expr param
	if len(def.Members) < 1 {
		t.Fatalf("Expected at least 1 member in calc body, got %d", len(def.Members))
	}

	// Check first member
	firstMem, ok := def.Members[0].(*ast.Membership)
	if !ok {
		t.Fatalf("Expected Membership, got %T", def.Members[0])
	}

	firstUsage, ok := firstMem.Member.(*ast.Usage)
	if !ok {
		t.Fatalf("Expected first to be Usage, got %T", firstMem.Member)
	}

	if firstUsage.Kind != ast.UsageExpr {
		t.Errorf("Expected UsageExpr, got %v", firstUsage.Kind)
	}

	if firstUsage.Ident.Name != "thenValue" {
		t.Errorf("Expected name 'thenValue', got %q", firstUsage.Ident.Name)
	}

	if firstUsage.Direction != ast.DirIn {
		t.Errorf("Expected direction In, got %v", firstUsage.Direction)
	}
}

// A trailing expression may start with a prefix operator, while an argument
// written after `->` without parentheses may not — a `-` there continues the
// invocation's result as a binary operand.
func TestTrailingExpressionPrefixOperators(t *testing.T) {
	trailing := func(t *testing.T, body string) ast.Node {
		t.Helper()
		src := "calc def C { in x; in xs; " + body + " }"
		p := New(source.New("prefix.sysml", []byte(src)))
		root := p.ParseFile()
		if len(p.Diagnostics) > 0 {
			t.Fatalf("unexpected diagnostics for %q: %v", src, p.Diagnostics)
		}
		def := root.Members[0].(*ast.Membership).Member.(*ast.Definition)
		last := def.Members[len(def.Members)-1]
		if m, ok := last.(*ast.Membership); ok {
			last = m.Member
		}
		return last
	}

	for _, tc := range []struct {
		name, body string
		op         ast.OperatorKind
	}{
		{"negation", "-x", ast.OpNeg},
		{"unary_plus", "+x", ast.OpPos},
		{"bit_not", "~x", ast.OpBitNot},
		{"logical_not", "not x", ast.OpNot},
		{"arrow_result_minus_one", "xs->size - 1", ast.OpSub},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := trailing(t, tc.body).(*ast.OperatorExpr)
			if !ok {
				t.Fatalf("%q parsed to %T, want *ast.OperatorExpr", tc.body, trailing(t, tc.body))
			}
			if got.Operator != tc.op {
				t.Errorf("%q operator = %v, want %v", tc.body, got.Operator, tc.op)
			}
		})
	}
}

// A trailing expression may begin with a name followed by a keyword binary
// operator; the keyword does not make the name a declaration.
func TestTrailingExpressionWordOperators(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		op         ast.OperatorKind
	}{
		{"and", "a and b", ast.OpConditionalAnd},
		{"or", "a or b", ast.OpConditionalOr},
		{"xor", "a xor b", ast.OpXor},
		{"implies", "a implies b", ast.OpImplies},
		{"as", "a as Real", ast.OpAs},
		{"istype", "a istype Real", ast.OpIsType},
		{"hastype", "a hastype Real", ast.OpHasType},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := "calc def C { in a; in b; " + tc.body + " }"
			p := New(source.New("word_op.sysml", []byte(src)))
			root := p.ParseFile()
			if len(p.Diagnostics) > 0 {
				t.Fatalf("unexpected diagnostics for %q: %v", src, p.Diagnostics)
			}
			def := root.Members[0].(*ast.Membership).Member.(*ast.Definition)
			last := def.Members[len(def.Members)-1]
			if m, ok := last.(*ast.Membership); ok {
				last = m.Member
			}
			got, ok := last.(*ast.OperatorExpr)
			if !ok {
				t.Fatalf("%q parsed to %T, want *ast.OperatorExpr", tc.body, last)
			}
			if got.Operator != tc.op {
				t.Errorf("%q operator = %v, want %v", tc.body, got.Operator, tc.op)
			}
		})
	}
}
