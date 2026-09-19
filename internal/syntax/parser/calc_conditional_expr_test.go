package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestCalcConditionalExpressionWithBodyCondition covers the `if` a calculation
// body may start with: a conditional expression whose condition holds a body
// expression must still be read as an expression, not as an if statement.
func TestCalcConditionalExpressionWithBodyCondition(t *testing.T) {
	bodies := []struct {
		name string
		body string
	}{
		{"implicit result", "if n > 0 ? 1 else 0"},
		{"implicit result over a body expression", "if xs->exists { it > n } ? 1 else 0"},
		{"implicit result over parentheses", "if (n > 0 and (n < 9)) ? 1 else 0"},
		{"implicit result over two body expressions", "if xs->exists { it > n } and xs->exists { it < n } ? 1 else 0"},
		{"body expression in an if statement", "if xs->exists { it > n } { return : Integer = 1; } return : Integer = 0;"},
		{"if statement before an implicit conditional result", "if n < 0 { return : Integer = 0; } if n > 5 ? 1 else 2"},
	}

	for _, tc := range bodies {
		t.Run(tc.name, func(t *testing.T) {
			src := "package P { calc def C { in n : Integer; in xs : Integer[*]; " + tc.body + " } }"
			p := New(source.New("t.sysml", []byte(src)))
			p.ParseFile()
			if len(p.Diagnostics) != 0 {
				t.Fatalf("parse diagnostics for %q: %v", tc.body, p.Diagnostics)
			}
		})
	}
}

// Both '?' operands are owned expressions per KerMLExpressions.xtext.
func TestCalcConditionalExpressionNestedInThenBranch(t *testing.T) {
	bodies := []string{
		"if n > 0 ? if n > 9 ? 9 else n else 0",
		"if n > 0 ? if n > 9 ? 9 else n else if n < -9 ? -9 else n",
	}
	for _, body := range bodies {
		src := "package P { calc def C { in n : Integer; " + body + " } }"
		p := New(source.New("t.sysml", []byte(src)))
		p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Fatalf("parse diagnostics for %q: %v", body, p.Diagnostics)
		}
	}
}
