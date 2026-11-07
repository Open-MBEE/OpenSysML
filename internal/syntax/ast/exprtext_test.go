package ast_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func parseExpr(t *testing.T, text string) ast.Node {
	t.Helper()
	p := parser.New(source.New("expr.sysml", []byte(text)))
	node := p.ParseExpression()
	if node == nil {
		t.Fatalf("parse %q: no expression", text)
	}
	return node
}

// TestExprPrinterGroupsByPrecedence checks the printed text carries every parenthesis the tree
// needs and none it does not, so it reads back as the tree it was spelled from.
func TestExprPrinterGroupsByPrecedence(t *testing.T) {
	cases := []struct{ in, want string }{
		{"a and (b or c)", "a and (b or c)"},
		{"(a and b) or c", "a and b or c"},
		{"a and b or c", "a and b or c"},
		{"x > 1.0 and (x < 0.0 or x > 5.0) and x < 3.0", "x > 1.0 and (x < 0.0 or x > 5.0) and x < 3.0"},
		{"(a + b) * c", "(a + b) * c"},
		{"a + b * c", "a + b * c"},
		{"a - (b - c)", "a - (b - c)"},
		{"(a - b) - c", "a - b - c"},
		{"a ** (b ** c)", "a ** b ** c"},
		{"(a ** b) ** c", "(a ** b) ** c"},
		{"not (a and b)", "not (a and b)"},
		{"not (a or b)", "not (a or b)"},
		{"not a and b", "not a and b"},
		{"-(a + b)", "-(a + b)"},
		{"-a + b", "-a + b"},
		{"(a or b) implies c", "a or b implies c"},
		{"a implies (b or c)", "a implies b or c"},
		{"(a implies b) implies c", "a implies b implies c"},
		{"a implies (b implies c)", "a implies (b implies c)"},
		{"a == (b and c)", "a == (b and c)"},
		{"(a + b).size()", "(a + b).size()"},
		{"(a + b) [m]", "(a + b) [m]"},
		{"if a ? b else c", "if a ? b else c"},
	}
	for _, tc := range cases {
		got := (ast.ExprPrinter{}).Text(parseExpr(t, tc.in))
		if got != tc.want {
			t.Errorf("%q: printed %q, want %q", tc.in, got, tc.want)
			continue
		}
		if again := (ast.ExprPrinter{}).Text(parseExpr(t, got)); again != got {
			t.Errorf("%q: %q reads back as %q", tc.in, got, again)
		}
	}
}
