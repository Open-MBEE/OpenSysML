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
		{"x @ Safety::Critical", "x @ Safety::Critical"},
		{"@Safety::Critical", "@Safety::Critical"},
		{"@@Metaclass", "@@Metaclass"},
		{"@A and not @@B", "@A and not @@B"},
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

// TestExprPrinterSpellsEveryExpressionForm checks the forms beyond operators and
// invocations print as the notation writes them and read back as the same text.
func TestExprPrinterSpellsEveryExpressionForm(t *testing.T) {
	cases := []struct{ in, want string }{
		{"parts.{in p; p.mass}", "parts.{in p; p.mass}"},
		{"parts.?{in p : Part; p.mass > 1}", "parts.?{in p : Part; p.mass > 1}"},
		{"(a + b).?{in x; x > 0}", "(a + b).?{in x; x > 0}"},
		{"parts.{in ref p; p}.mass", "parts.{in ref p; p}.mass"},
		{"xs->exists {in x; x > 0}", "xs->exists {in x; x > 0}"},
		{"new Point(1, 2)", "new Point(1, 2)"},
		{"new Geometry::Point(x = 1, y = 2)", "new Geometry::Point(x = 1, y = 2)"},
		{"new Point()", "new Point()"},
		{"(1, 2, a + b)", "(1, 2, a + b)"},
		{"(a, b)->size()", "(a, b)->size()"},
		{"vehicle.metadata", "vehicle.metadata"},
		{"Fleet::vehicle.metadata", "Fleet::vehicle.metadata"},
		{"(as Vehicle)", "(as Vehicle)"},
		{"(as Fleet::Vehicle[1..*])", "(as Fleet::Vehicle[1..*])"},
		{"(as Vehicle[2])", "(as Vehicle[2])"},
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

// TestExprPrinterLeavesADeclaringBodyToOther checks a body that declares a member
// is handed to Other, and has no spelling without it, rather than printed without the member.
func TestExprPrinterLeavesADeclaringBodyToOther(t *testing.T) {
	node := parseExpr(t, "xs->collect {in x; private attribute y = x; y}")
	if got := (ast.ExprPrinter{}).Text(node); got != "" {
		t.Errorf("printed %q, want no spelling", got)
	}
	other := ast.ExprPrinter{Other: func(ast.Node) string { return "<body>" }}
	if got, want := other.Text(node), "xs->collect <body>"; got != want {
		t.Errorf("printed %q, want %q", got, want)
	}
}
