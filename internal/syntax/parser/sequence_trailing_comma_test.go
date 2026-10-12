package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A SequenceExpression admits one trailing comma that adds no operand
// (KerMLExpressions.xtext SequenceExpression: `OwnedExpression ','` is an
// alternative of its own), in every position the grammar reuses the production.
func TestSequenceTrailingCommaAddsNoOperand(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"pair", "(a, b,)", "(SequenceExpr\n  (FeatureReference name=\"a\")\n  (FeatureReference name=\"b\"))"},
		{"pair_split", "(\n\ta,\n\tb,\n)", "(SequenceExpr\n  (FeatureReference name=\"a\")\n  (FeatureReference name=\"b\"))"},
		{"one", "(a,)", "(FeatureReference name=\"a\")"},
		{"plain", "(a)", "(FeatureReference name=\"a\")"},
		{"hash_index", "s#(1, 2,)", "(IndexExpr\n  (FeatureReference name=\"s\")\n  (SequenceExpr\n    (LiteralInteger value=\"1\")\n    (LiteralInteger value=\"2\")))"},
		{"hash_index_one", "s#(1,)", "(IndexExpr\n  (FeatureReference name=\"s\")\n  (LiteralInteger value=\"1\"))"},
		{"bracket_index", "s[1, 2,]", "(IndexExpr bracket=true\n  (FeatureReference name=\"s\")\n  (SequenceExpr\n    (LiteralInteger value=\"1\")\n    (LiteralInteger value=\"2\")))"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(source.New(tt.name+".sysml", []byte(tt.src)))
			expr := p.ParseExpression()
			if len(p.Diagnostics) != 0 {
				t.Fatalf("diagnostics: %v", p.Diagnostics)
			}
			if got := ast.Dump(expr); got != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

// Only one comma may trail, and only an operand may precede it; an argument
// list admits no trailing comma at all (KerMLExpressions.xtext
// PositionalArgumentList).
func TestSequenceTrailingCommaNegative(t *testing.T) {
	for _, src := range []string{"(a,,)", "(,)", "s#(,)", "s#(1,,)", "s[,]", "f(a,)", "f(x = a,)"} {
		t.Run(src, func(t *testing.T) {
			p := New(source.New("t.sysml", []byte(src)))
			p.ParseExpression()
			if len(p.Diagnostics) == 0 {
				t.Fatalf("%q parsed without a diagnostic", src)
			}
		})
	}
}
