package ast

import (
	"strings"
	"testing"
)

func TestDumpLiteral(t *testing.T) {
	got := Dump(&LiteralInteger{Value: "42"})
	want := `(LiteralInteger value="42")`
	if strings.TrimSpace(got) != want {
		t.Fatalf("Dump = %q, want %q", got, want)
	}
}

func TestDumpOperatorExprNested(t *testing.T) {
	e := &OperatorExpr{
		Operator: OpAdd,
		Operands: []Node{
			&LiteralInteger{Value: "1"},
			&OperatorExpr{Operator: OpMul, Operands: []Node{
				&LiteralInteger{Value: "2"},
				&LiteralInteger{Value: "3"},
			}},
		},
	}
	want := strings.Join([]string{
		`(OperatorExpr operator="+"`,
		`  (LiteralInteger value="1")`,
		`  (OperatorExpr operator="*"`,
		`    (LiteralInteger value="2")`,
		`    (LiteralInteger value="3")))`,
	}, "\n")
	if got := strings.TrimSpace(Dump(e)); got != want {
		t.Fatalf("Dump =\n%s\nwant\n%s", got, want)
	}
}

func TestDumpQualifiedName(t *testing.T) {
	qn := &QualifiedName{Parts: []NameSegment{{Text: "A"}, {Text: "B"}}}
	got := strings.TrimSpace(Dump(&FeatureReference{Name: qn}))
	want := `(FeatureReference name="A::B")`
	if got != want {
		t.Fatalf("Dump = %q, want %q", got, want)
	}
}
