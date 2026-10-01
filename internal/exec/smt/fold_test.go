package smt

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/solve"
)

// render writes a term as an S-expression over the operators the folds emit.
func render(t *solve.Term) string {
	switch t.Op {
	case solve.OpBool:
		return fmt.Sprint(t.Bool)
	case solve.OpInt:
		return fmt.Sprint(t.Int)
	case solve.OpVar:
		return t.Var.Name
	}
	ops := map[solve.Op]string{
		solve.OpAnd: "and", solve.OpOr: "or", solve.OpNot: "not", solve.OpImplies: "=>",
		solve.OpIte: "ite", solve.OpEq: "=", solve.OpAdd: "+", solve.OpSub: "-",
		solve.OpGt: ">", solve.OpGe: ">=",
	}
	args := make([]string, 0, len(t.Args))
	for _, a := range t.Args {
		args = append(args, render(a))
	}
	return "(" + ops[t.Op] + " " + strings.Join(args, " ") + ")"
}

// TestFoldDecidesLiteralOperands: the folding constructors decide what the
// literals decide and leave every other term to the solver.
func TestFoldDecidesLiteralOperands(t *testing.T) {
	x := solve.VarTerm(boolVar("x"))
	y := solve.VarTerm(boolVar("y"))
	n := solve.VarTerm(intVar("n"))
	yes, no := solve.BoolTerm(true), solve.BoolTerm(false)
	cases := []struct {
		name string
		got  *solve.Term
		want string
	}{
		{"and drops true", and(yes, x, y), "(and x y)"},
		{"and decides false", and(x, no), "false"},
		{"or drops false", or(no, x), "x"},
		{"or decides true", or(x, yes), "true"},
		{"not literal", not(no), "true"},
		{"implies false antecedent", implies(no, x), "true"},
		{"implies true antecedent", implies(yes, x), "x"},
		{"implies false consequent", implies(x, no), "(not x)"},
		{"ite decided", ite(no, x, y), "y"},
		{"ite same branches", ite(x, n, n), "n"},
		{"eq same variable", eq(n, n), "true"},
		{"eq distinct literals", eq(solve.IntTerm(1), solve.IntTerm(2)), "false"},
		{"eq true", eq(yes, x), "x"},
		{"eq false", eq(x, no), "(not x)"},
		{"eq open", eq(x, y), "(= x y)"},
		{"add literals", add(solve.IntTerm(2), solve.IntTerm(3)), "5"},
		{"add zero", add(solve.IntTerm(0), n), "n"},
		{"add open", add(n, solve.IntTerm(1)), "(+ n 1)"},
		{"sub literals", sub(solve.IntTerm(2), solve.IntTerm(3)), "-1"},
		{"sub zero", sub(n, solve.IntTerm(0)), "n"},
		{"gt literals", gt(solve.IntTerm(2), solve.IntTerm(3)), "false"},
		{"ge literals", ge(solve.IntTerm(3), solve.IntTerm(3)), "true"},
		{"ge open", ge(n, solve.IntTerm(1)), "(>= n 1)"},
	}
	for _, c := range cases {
		if got := render(c.got); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
}

// TestFoldIsExactBeyondInt64: a sum or difference outside int64 folds to the
// exact Integer, as the solver's unbounded Int would compute it, never a wrap.
func TestFoldIsExactBeyondInt64(t *testing.T) {
	largest, smallest := solve.IntTerm(math.MaxInt64), solve.IntTerm(math.MinInt64)
	one := solve.IntTerm(1)
	for _, c := range []struct {
		name string
		got  *solve.Term
		want string
	}{
		{"max plus one", add(largest, one), "9223372036854775808"},
		{"min minus one", sub(smallest, one), "-9223372036854775809"},
		{"zero minus min", sub(solve.IntTerm(0), smallest), "9223372036854775808"},
		{"min plus min", add(smallest, smallest), "-18446744073709551616"},
		{"back within int64", sub(add(largest, one), one), "9223372036854775807"},
	} {
		if c.got.Op != solve.OpInt || c.got.IntBig().String() != c.want {
			t.Errorf("%s folded to %s, want %s", c.name, render(c.got), c.want)
		}
	}
	if got := add(largest, solve.IntTerm(-1)); got.Op != solve.OpInt || got.Int != math.MaxInt64-1 || got.Big != nil {
		t.Errorf("max minus one: %s", render(got))
	}
	if got := sub(add(largest, one), one); got.Big != nil || got.Int != math.MaxInt64 {
		t.Errorf("a fold back within int64 keeps a big literal: %+v", got)
	}
}
