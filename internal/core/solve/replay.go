package solve

import (
	"fmt"
	"math"
	"math/big"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// Rounded reports whether the query asserts or optimizes a value the evaluator
// computes in float64 — a real operation, a widened integer, or a real literal
// with no exact float64 — whose rounding the exact-real encoding does not model.
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

// roundedTerm reports whether the term computes a value float64 may round: real
// arithmetic, a widened integer (rounded beyond 2^53), or a real literal whose
// decimal has no exact float64.
func roundedTerm(t *Term) bool {
	switch t.Op {
	case OpReal:
		_, exact := t.Real.Float64()
		if !exact {
			return true
		}
	case OpAdd, OpSub, OpMul, OpDiv, OpNeg:
		if t.Sort.Kind == SortReal {
			return true
		}
	case OpToReal:
		return true
	}
	for _, arg := range t.Args {
		if roundedTerm(arg) {
			return true
		}
	}
	return false
}

// replayValue is one value a replayed term yields, in the evaluator's own
// representations: int64, float64, bool, or a string naming a literal or a
// datatype value.
type replayValue struct {
	kind SortKind
	b    bool
	i    int64
	f    float64
	s    string
}

// replayWitness re-runs the query's assertions through the evaluator's own
// arithmetic on the witness, reporting ok=false with why when the evaluator
// does not confirm what the solver answered.
func replayWitness(q *Query, model []Assignment) (bool, string) {
	env := make(map[string]replayValue, len(model))
	for _, a := range model {
		val, err := witnessValue(a)
		if err != nil {
			return false, fmt.Sprintf("the evaluator cannot hold the value the solver chose for %s: %v", a.Var.Name, err)
		}
		env[a.Var.Name] = val
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

// witnessValue reads one assignment back into the evaluator's representation,
// reporting a value the evaluator cannot hold — an Integer outside int64, a
// Real with no finite float64 — as an error.
func witnessValue(a Assignment) (replayValue, error) {
	value, err := DecodeValue(a)
	if err != nil {
		return replayValue{}, err
	}
	switch value.Kind {
	case SortBool:
		return replayValue{kind: SortBool, b: value.Bool}, nil
	case SortString, SortDatatype:
		return replayValue{kind: value.Kind, s: value.Text}, nil
	case SortInt:
		if !value.Number.Num().IsInt64() {
			return replayValue{}, fmt.Errorf("%s is outside the Integer range", value.Number.Num().String())
		}
		return replayValue{kind: SortInt, i: value.Number.Num().Int64()}, nil
	case SortReal:
		// The evaluator holds the nearest float64, which is where a witness
		// only the exact encoding can hold is caught by the replay.
		f, _ := value.Number.Float64()
		if math.IsInf(f, 0) || math.IsNaN(f) {
			return replayValue{}, fmt.Errorf("%s is outside the Real range", a.Value)
		}
		return replayValue{kind: SortReal, f: f}, nil
	}
	return replayValue{}, fmt.Errorf("unreadable value %s", a.Raw)
}

// replayTerm evaluates a term as the runtime evaluator computes it: int64 and
// float64 arithmetic, integers widened to float64 where a real meets them. An
// evaluation the evaluator would report — a zero divisor, an overflow — is an
// error, never a verdict.
func replayTerm(t *Term, env map[string]replayValue) (replayValue, error) {
	switch t.Op {
	case OpBool:
		return replayValue{kind: SortBool, b: t.Bool}, nil
	case OpInt:
		return replayValue{kind: SortInt, i: t.Int}, nil
	case OpReal:
		// The evaluator parses a real literal to the nearest float64, which
		// Float64 on the exact rational also is.
		f, _ := t.Real.Float64()
		return replayValue{kind: SortReal, f: f}, nil
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
			if val.i == math.MinInt64 {
				return replayValue{}, fmt.Errorf("-%d exceeds the Integer range", val.i)
			}
			return replayValue{kind: SortInt, i: -val.i}, nil
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
		// float64(int64) is the widening the evaluator applies where a real
		// meets an integer, which rounds beyond 2^53.
		return replayValue{kind: SortReal, f: float64(val.i)}, nil
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

// replayEquality compares as the evaluator compares: numbers through float64,
// which is what `valueEqual` delegates to, and everything else by identity.
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
		eq = left.asReal() == right.asReal()
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

// replayComparison orders as the evaluator orders: two integers exactly, a
// mixed pair through float64.
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
	var res bool
	if left.kind == SortInt && right.kind == SortInt {
		switch t.Op {
		case OpLt:
			res = left.i < right.i
		case OpLe:
			res = left.i <= right.i
		case OpGt:
			res = left.i > right.i
		case OpGe:
			res = left.i >= right.i
		}
		return replayValue{kind: SortBool, b: res}, nil
	}
	lf, rf := left.asReal(), right.asReal()
	switch t.Op {
	case OpLt:
		res = lf < rf
	case OpLe:
		res = lf <= rf
	case OpGt:
		res = lf > rf
	case OpGe:
		res = lf >= rf
	}
	return replayValue{kind: SortBool, b: res}, nil
}

// replayArithmetic computes as the evaluator computes: int64 with a reported
// overflow, an integer quotient as the exact ratio rounded once, and float64
// everywhere a real takes part, with a non-finite result reported.
func replayArithmetic(t *Term, env map[string]replayValue) (replayValue, error) {
	// A whole-number quotient is the exact ratio rounded once to float64, not
	// float64 division of widened operands.
	if t.Op == OpDiv && t.IntRatio {
		return replayRatio(t, env)
	}
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
	if left.kind == SortInt && right.kind == SortInt {
		if t.Op == OpDiv {
			q, ok := semantics.IntQuotient(left.i, right.i)
			if !ok {
				return replayValue{}, fmt.Errorf("division by zero")
			}
			return replayValue{kind: SortReal, f: q}, nil
		}
		res, ok := semantics.IntArith(astOp, left.i, right.i)
		if !ok {
			return replayValue{}, fmt.Errorf("%d %s %d exceeds the Integer range", left.i, smtOps[t.Op], right.i)
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
	if right.i == 0 {
		return replayValue{}, fmt.Errorf("division by zero")
	}
	if left.i == math.MinInt64 && right.i == -1 {
		return replayValue{}, fmt.Errorf("%d div %d exceeds the Integer range", left.i, right.i)
	}
	q := left.i / right.i
	if left.i%right.i < 0 {
		if right.i > 0 {
			q--
		} else {
			q++
		}
	}
	return replayValue{kind: SortInt, i: q}, nil
}

// replayRatio computes a whole-number quotient as the evaluator does: the exact
// ratio of its integer operands, rounded once to the nearest float64.
func replayRatio(t *Term, env map[string]replayValue) (replayValue, error) {
	left, err := exactRat(t.Args[0], env)
	if err != nil {
		return replayValue{}, err
	}
	right, err := exactRat(t.Args[1], env)
	if err != nil {
		return replayValue{}, err
	}
	if right.Sign() == 0 {
		return replayValue{}, fmt.Errorf("division by zero")
	}
	f, _ := new(big.Rat).Quo(left, right).Float64()
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return replayValue{}, fmt.Errorf("the quotient is not a finite Real")
	}
	return replayValue{kind: SortReal, f: f}, nil
}

// exactRat reads a ratio operand exactly, stripping the encoding's Int-to-Real
// widening so no float64 rounding precedes the quotient's own.
func exactRat(t *Term, env map[string]replayValue) (*big.Rat, error) {
	if t.Op == OpToReal {
		t = t.Args[0]
	}
	if t.Op == OpReal {
		return t.Real, nil
	}
	val, err := replayTerm(t, env)
	if err != nil {
		return nil, err
	}
	if val.kind == SortInt {
		return new(big.Rat).SetInt64(val.i), nil
	}
	if val.kind == SortReal {
		return new(big.Rat).SetFloat64(val.f), nil
	}
	return nil, fmt.Errorf("a quotient operand yielded no number")
}

// numeric reports whether the value is an integer or a real.
func (v replayValue) numeric() bool { return v.kind == SortInt || v.kind == SortReal }

// asReal is the value as the evaluator widens it where a real meets it.
func (v replayValue) asReal() float64 {
	if v.kind == SortInt {
		return float64(v.i)
	}
	return v.f
}
