package solve

import (
	"errors"
	"fmt"
	"math"
	"math/big"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Rounded reports whether the query asserts or optimizes a value the evaluator
// computes in binary64 — arithmetic a Real takes part in — whose rounding the
// exact-real encoding does not model. Integer and Rational arithmetic is exact.
func (q *Query) Rounded() bool {
	for _, a := range q.Assertions {
		if roundedTerm(a.Term) {
			return true
		}
	}
	for _, o := range q.Objectives {
		if roundedTerm(o.Term) {
			return true
		}
	}
	return false
}

// Rounded reports whether the conflict rests on a condition the evaluator
// computes in float64; a conflict with no members to inspect counts.
func (c *Core) Rounded() bool {
	if c == nil || len(c.Members) == 0 {
		return true
	}
	for _, m := range c.Members {
		if roundedTerm(m.Term) {
			return true
		}
	}
	return false
}

// roundedTerm reports whether the term computes a value binary64 may round:
// arithmetic a Real takes part in, which also rounds its exact operands once.
func roundedTerm(t *Term) bool {
	switch t.Op {
	case OpAdd, OpSub, OpMul, OpDiv:
		if t.Sort.Kind == SortReal && t.Binary64() {
			return true
		}
	}
	for _, arg := range t.Args {
		if roundedTerm(arg) {
			return true
		}
	}
	return false
}

// replayValue is one value a replayed term yields, in the evaluator's own
// representations: an exact Integer or Rational, a binary64 Real, a bool, or a
// string naming a literal or a datatype value.
type replayValue struct {
	kind SortKind
	b    bool
	// i is an Integer, or with exact a Rational of the Real sort.
	i     semantics.Value
	exact bool
	f     float64
	s     string
}

// exactReal is a Real-sorted value the evaluator holds exactly, as a Rational.
func exactReal(v semantics.Value) replayValue {
	return replayValue{kind: SortReal, i: v, exact: true}
}

// replayWitness re-runs the query's assertions through the evaluator's own
// arithmetic on the witness, reporting ok=false with why when the evaluator
// does not confirm what the solver answered.
func replayWitness(q *Query, model []Assignment) (bool, string) {
	values := make(map[string]ModelValue, len(model))
	for _, a := range model {
		value, err := DecodeValue(a)
		if err != nil {
			return false, fmt.Sprintf("the evaluator cannot hold the value the solver chose for %s: %v", a.Var.Name, err)
		}
		values[a.Var.Name] = value
	}
	return q.Confirm(values)
}

// Confirm re-runs the query's assertions through the evaluator's own arithmetic on
// values of its variables by name, as a solver's model is confirmed before `sat` is
// claimed: ok=false with why when a variable has no value, a value the evaluator
// cannot hold, or an assertion that does not evaluate to true.
func (q *Query) Confirm(values map[string]ModelValue) (bool, string) {
	env := make(map[string]replayValue, len(values))
	for _, v := range q.Vars {
		value, ok := values[v.Name]
		if !ok {
			return false, fmt.Sprintf("the witness gives %s no value", v.Name)
		}
		if value.Kind != v.Sort.Kind {
			return false, fmt.Sprintf("the witness gives %s no value of sort %s", v.Name, v.Sort.Name)
		}
		if value.Kind == SortDatatype && !contains(v.Sort.Values, value.Text) {
			return false, fmt.Sprintf("the witness gives %s %s, not a value of %s", v.Name, value.Text, v.Sort.Name)
		}
		val, err := evaluatorValue(value, v.Binary64)
		if err != nil {
			return false, fmt.Sprintf("the evaluator cannot hold the value the solver chose for %s: %v", v.Name, err)
		}
		env[v.Name] = val
	}
	for _, assertion := range q.Assertions {
		val, err := replayTerm(assertion.Term, env)
		if err != nil {
			return false, fmt.Sprintf("the evaluator does not decide the witness: %v (%s `%s`)",
				err, assertion.From.Role, assertion.From.Condition)
		}
		if val.kind != SortBool {
			return false, fmt.Sprintf("replaying the %s `%s` yielded no boolean",
				assertion.From.Role, assertion.From.Condition)
		}
		if !val.b {
			return false, fmt.Sprintf("the evaluator's floating-point arithmetic rejects the witness at the %s `%s`",
				assertion.From.Role, assertion.From.Condition)
		}
	}
	return true, ""
}

// evaluatorValue holds one model value in the evaluator's representation,
// exactly unless binary64, reporting a value the evaluator cannot hold — a
// Real with no finite float64 — as an error.
func evaluatorValue(value ModelValue, binary64 bool) (replayValue, error) {
	switch value.Kind {
	case SortBool:
		return replayValue{kind: SortBool, b: value.Bool}, nil
	case SortString, SortDatatype:
		return replayValue{kind: value.Kind, s: value.Text}, nil
	case SortInt:
		if !value.Number.IsInt() {
			return replayValue{}, fmt.Errorf("%s is no Integer", value.Number.RatString())
		}
		return replayValue{kind: SortInt, i: semantics.BigIntValue(new(big.Int).Set(value.Number.Num()))}, nil
	case SortReal:
		if !binary64 {
			return exactReal(semantics.RatValue(value.Number)), nil
		}
		// The evaluator holds the nearest float64, which is where a witness
		// only the exact encoding can hold is caught by the replay.
		f, _ := value.Number.Float64()
		if math.IsInf(f, 0) || math.IsNaN(f) {
			return replayValue{}, fmt.Errorf("%s is outside the Real range", value.Number.FloatString(6))
		}
		return replayValue{kind: SortReal, f: f}, nil
	}
	return replayValue{}, fmt.Errorf("a value of no sort")
}

// replayTerm evaluates a term as the runtime evaluator computes it: exact
// Integer and Rational arithmetic, and binary64 arithmetic where a Real takes
// part, its exact operands rounded once to float64. An evaluation the evaluator would report — a zero divisor, an
// Integer past the size budget — is an error, never a verdict.
func replayTerm(t *Term, env map[string]replayValue) (replayValue, error) {
	switch t.Op {
	case OpBool:
		return replayValue{kind: SortBool, b: t.Bool}, nil
	case OpInt:
		return replayValue{kind: SortInt, i: semantics.BigIntValue(t.IntBig())}, nil
	case OpReal:
		// A decimal literal is an exact Rational.
		return exactReal(semantics.RatValue(t.Real)), nil
	case OpString:
		return replayValue{kind: SortString, s: t.Str}, nil
	case OpValue:
		return replayValue{kind: SortDatatype, s: t.Str}, nil
	case OpVar:
		val, ok := env[t.Var.Name]
		if !ok {
			return replayValue{}, fmt.Errorf("no value for %s", t.Var.Name)
		}
		return val, nil
	case OpNot:
		val, err := replayBool(t.Args[0], env)
		if err != nil {
			return replayValue{}, err
		}
		return replayValue{kind: SortBool, b: !val}, nil
	case OpAnd, OpOr:
		return replayJunction(t, env)
	case OpXor, OpImplies:
		left, err := replayBool(t.Args[0], env)
		if err != nil {
			return replayValue{}, err
		}
		if t.Op == OpImplies && !left {
			return replayValue{kind: SortBool, b: true}, nil
		}
		right, err := replayBool(t.Args[1], env)
		if err != nil {
			return replayValue{}, err
		}
		if t.Op == OpXor {
			return replayValue{kind: SortBool, b: left != right}, nil
		}
		return replayValue{kind: SortBool, b: right}, nil
	case OpEq, OpNe:
		return replayEquality(t, env)
	case OpLt, OpLe, OpGt, OpGe:
		return replayComparison(t, env)
	case OpAdd, OpSub, OpMul, OpDiv:
		return replayArithmetic(t, env)
	case OpIntDiv:
		return replayIntDiv(t, env)
	case OpNeg:
		val, err := replayTerm(t.Args[0], env)
		if err != nil {
			return replayValue{}, err
		}
		if val.kind == SortInt {
			return replayValue{kind: SortInt, i: semantics.IntNeg(val.i)}, nil
		}
		if val.exact {
			return exactReal(semantics.RatNeg(val.i)), nil
		}
		if val.kind == SortReal {
			return replayValue{kind: SortReal, f: -val.f}, nil
		}
	case OpIte:
		cond, err := replayBool(t.Args[0], env)
		if err != nil {
			return replayValue{}, err
		}
		if cond {
			return replayTerm(t.Args[1], env)
		}
		return replayTerm(t.Args[2], env)
	case OpToReal:
		val, err := replayTerm(t.Args[0], env)
		if err != nil {
			return replayValue{}, err
		}
		if val.kind != SortInt {
			return replayValue{}, fmt.Errorf("widening a non-integer")
		}
		// An Integer is a Rational exactly; arithmetic a Real takes part in
		// rounds it then.
		return exactReal(val.i), nil
	}
	return replayValue{}, fmt.Errorf("the replay does not define this term")
}

// replayBool evaluates a term that must yield a boolean.
func replayBool(t *Term, env map[string]replayValue) (bool, error) {
	val, err := replayTerm(t, env)
	if err != nil {
		return false, err
	}
	if val.kind != SortBool {
		return false, fmt.Errorf("a boolean operand yielded none")
	}
	return val.b, nil
}

// replayJunction evaluates a conjunction or disjunction as the evaluator does,
// deciding on an earlier operand where it can so a guarded operand it rules out
// is never evaluated.
func replayJunction(t *Term, env map[string]replayValue) (replayValue, error) {
	deciding := t.Op == OpOr
	for _, arg := range t.Args {
		val, err := replayBool(arg, env)
		if err != nil {
			return replayValue{}, err
		}
		if val == deciding {
			return replayValue{kind: SortBool, b: deciding}, nil
		}
	}
	return replayValue{kind: SortBool, b: !deciding}, nil
}

// replayEquality compares as the evaluator compares: numbers by their exact
// values, a binary64 Real by the exact value it holds, and everything else by
// identity.
func replayEquality(t *Term, env map[string]replayValue) (replayValue, error) {
	left, err := replayTerm(t.Args[0], env)
	if err != nil {
		return replayValue{}, err
	}
	right, err := replayTerm(t.Args[1], env)
	if err != nil {
		return replayValue{}, err
	}
	var eq bool
	switch {
	case left.numeric() && right.numeric():
		eq = compareNumbers(left, right) == 0
	case left.kind == SortBool && right.kind == SortBool:
		eq = left.b == right.b
	case left.kind == right.kind:
		eq = left.s == right.s
	default:
		return replayValue{}, fmt.Errorf("comparing values of different kinds")
	}
	if t.Op == OpNe {
		eq = !eq
	}
	return replayValue{kind: SortBool, b: eq}, nil
}

// replayComparison orders as the evaluator orders: by exact value, a binary64
// Real by the exact value it holds.
func replayComparison(t *Term, env map[string]replayValue) (replayValue, error) {
	left, err := replayTerm(t.Args[0], env)
	if err != nil {
		return replayValue{}, err
	}
	right, err := replayTerm(t.Args[1], env)
	if err != nil {
		return replayValue{}, err
	}
	if left.kind == SortString && right.kind == SortString {
		var res bool
		switch t.Op {
		case OpLt:
			res = left.s < right.s
		case OpLe:
			res = left.s <= right.s
		case OpGt:
			res = left.s > right.s
		case OpGe:
			res = left.s >= right.s
		}
		return replayValue{kind: SortBool, b: res}, nil
	}
	if !left.numeric() || !right.numeric() {
		return replayValue{}, fmt.Errorf("ordering non-numbers")
	}
	res, _ := semantics.OrderSatisfies(map[Op]ast.OperatorKind{OpLt: ast.OpLt, OpLe: ast.OpLe, OpGt: ast.OpGt, OpGe: ast.OpGe}[t.Op], compareNumbers(left, right))
	return replayValue{kind: SortBool, b: res}, nil
}

// replayArithmetic computes as the evaluator computes: Integers and Rationals
// exactly within the size budget, an Integer quotient as the exact Rational, and
// binary64 where a Real takes part, with a non-finite result reported.
func replayArithmetic(t *Term, env map[string]replayValue) (replayValue, error) {
	left, err := replayTerm(t.Args[0], env)
	if err != nil {
		return replayValue{}, err
	}
	right, err := replayTerm(t.Args[1], env)
	if err != nil {
		return replayValue{}, err
	}
	if !left.numeric() || !right.numeric() {
		return replayValue{}, fmt.Errorf("arithmetic on non-numbers")
	}
	astOp := map[Op]ast.OperatorKind{OpAdd: ast.OpAdd, OpSub: ast.OpSub, OpMul: ast.OpMul, OpDiv: ast.OpDiv}[t.Op]
	if left.isExact() && right.isExact() && (t.Op == OpDiv || left.kind != SortInt || right.kind != SortInt) {
		res, err := semantics.RatArith(astOp, left.i, right.i, semantics.DefaultMaxIntegerBits)
		if errors.Is(err, semantics.ErrDivisionByZero) {
			return replayValue{}, fmt.Errorf("division by zero")
		}
		if err != nil {
			return replayValue{}, err
		}
		return exactReal(res), nil
	}
	if left.kind == SortInt && right.kind == SortInt {
		res, err := semantics.IntArith(astOp, left.i, right.i, semantics.DefaultMaxIntegerBits)
		if err != nil {
			return replayValue{}, err
		}
		return replayValue{kind: SortInt, i: res}, nil
	}
	res, ok := semantics.RealArith(astOp, left.asReal(), right.asReal())
	if !ok {
		return replayValue{}, fmt.Errorf("division by zero")
	}
	if math.IsInf(res, 0) || math.IsNaN(res) {
		return replayValue{}, fmt.Errorf("the result is not a finite Real")
	}
	return replayValue{kind: SortReal, f: res}, nil
}

// replayIntDiv computes SMT-LIB's Euclidean integer division, which TruncDiv
// only applies to a non-negative dividend it builds itself.
func replayIntDiv(t *Term, env map[string]replayValue) (replayValue, error) {
	left, err := replayTerm(t.Args[0], env)
	if err != nil {
		return replayValue{}, err
	}
	right, err := replayTerm(t.Args[1], env)
	if err != nil {
		return replayValue{}, err
	}
	if left.kind != SortInt || right.kind != SortInt {
		return replayValue{}, fmt.Errorf("integer division on non-integers")
	}
	if right.i.IntSign() == 0 {
		return replayValue{}, fmt.Errorf("division by zero")
	}
	// big.Int's Div is Euclidean, as SMT-LIB's div is.
	q := new(big.Int).Div(left.i.BigInt(), right.i.BigInt())
	return replayValue{kind: SortInt, i: semantics.BigIntValue(q)}, nil
}

// numeric reports whether the value is an integer or a real.
func (v replayValue) numeric() bool { return v.kind == SortInt || v.kind == SortReal }

// isExact reports whether the value is an Integer or a Rational.
func (v replayValue) isExact() bool { return v.kind == SortInt || v.exact }

// asReal is the value as the evaluator rounds it where a Real meets it.
func (v replayValue) asReal() float64 {
	if v.isExact() {
		return v.i.AsReal()
	}
	return v.f
}

// compareNumbers orders two numbers as the evaluator's comparison does: exact
// values exactly, a Rational against a binary64 at Real precision.
func compareNumbers(a, b replayValue) int {
	switch {
	case a.isExact() && b.isExact():
		return semantics.CompareRat(a.i, b.i)
	case a.isExact():
		return semantics.CompareReal(a.i, b.f)
	case b.isExact():
		return -semantics.CompareReal(b.i, a.f)
	case a.f < b.f:
		return -1
	case a.f > b.f:
		return 1
	}
	return 0
}
