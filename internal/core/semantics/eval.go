package semantics

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

var (
	// ErrArithmeticDomain reports operands an operation is not defined for, such
	// as a negative base raised to a fractional exponent.
	ErrArithmeticDomain = errors.New("arithmetic domain error")

	// ErrArithmeticOverflow reports a result outside the range of the kind it
	// would have: an Integer that does not fit int64, or a non-finite Real.
	ErrArithmeticOverflow = errors.New("arithmetic overflow")

	// ErrRealNotation reports text that is not decimal Real notation, such as
	// NaN, an infinity or a hexadecimal float.
	ErrRealNotation = errors.New("not decimal Real notation")
)

// ParseReal reads decimal Real notation as a finite binary64 Real. Any other
// notation is ErrRealNotation; a magnitude that overflows to an infinity, or
// a nonzero one that underflows to zero, is ErrArithmeticOverflow.
func ParseReal(text string) (float64, error) {
	if !isRealNotation(text) {
		return 0, ErrRealNotation
	}
	x, err := strconv.ParseFloat(text, 64)
	if err != nil || (x == 0 && !isZeroNotation(text)) {
		return 0, ErrArithmeticOverflow
	}
	return x, nil
}

// isRealNotation reports whether text is decimal Real notation: an optional
// sign, digits with an optional fraction, and an optional decimal exponent.
func isRealNotation(text string) bool {
	i := 0
	if i < len(text) && (text[i] == '+' || text[i] == '-') {
		i++
	}
	digits := 0
	for i < len(text) && isDigit(text[i]) {
		i, digits = i+1, digits+1
	}
	if i < len(text) && text[i] == '.' {
		i++
		for i < len(text) && isDigit(text[i]) {
			i, digits = i+1, digits+1
		}
	}
	if digits == 0 {
		return false
	}
	if i < len(text) && (text[i] == 'e' || text[i] == 'E') {
		i++
		if i < len(text) && (text[i] == '+' || text[i] == '-') {
			i++
		}
		exponent := 0
		for i < len(text) && isDigit(text[i]) {
			i, exponent = i+1, exponent+1
		}
		if exponent == 0 {
			return false
		}
	}
	return i == len(text)
}

func isDigit(c byte) bool { return '0' <= c && c <= '9' }

// isZeroNotation reports whether decimal notation denotes zero: no significand digit is nonzero.
func isZeroNotation(text string) bool {
	significand, _, _ := strings.Cut(strings.ToLower(text), "e")
	return !strings.ContainsAny(significand, "123456789")
}

// ValueKind discriminates a model-level constant value.
type ValueKind int

const (
	ValInvalid ValueKind = iota
	ValInt
	ValReal
	ValBool
	ValInfinity // the `*` bound / unbounded value
)

// Value is a model-level-evaluated constant. Only the field selected by Kind is
// meaningful. This is a deliberately small subset: the constraint checks that
// need evaluation (multiplicity bounds, some guards) operate over integers,
// reals, booleans, and the infinity bound.
type Value struct {
	Kind ValueKind
	Int  int64
	Real float64
	Bool bool
}

// IsNumeric reports whether the value is an integer or a real.
func (v Value) IsNumeric() bool { return v.Kind == ValInt || v.Kind == ValReal }

// WholeNumber returns the value as an Integer when it is one: an integer, or a
// finite real with no fractional part within the Integer range (4 / 2 is 2.0).
func (v Value) WholeNumber() (int64, bool) {
	switch v.Kind {
	case ValInt:
		return v.Int, true
	case ValReal:
		// MaxInt64 has no float64; 2^63 is the next value up and is out of range.
		if v.Real != math.Trunc(v.Real) || v.Real >= -float64(math.MinInt64) || v.Real < math.MinInt64 {
			return 0, false
		}
		return int64(v.Real), true
	}
	return 0, false
}

// IsUnbounded reports whether the value is the unbounded `*`.
func (v Value) IsUnbounded() bool { return v.Kind == ValInfinity }

// UnboundedOrder compares l and r where at least one is the unbounded `*`: `*`
// exceeds every finite number and equals itself. ok is false when the other
// operand is neither a number nor `*`, since nothing orders it against one.
func UnboundedOrder(l, r Value) (order int, ok bool) {
	ordinal := func(v Value) (int, bool) {
		switch {
		case v.IsUnbounded():
			return 1, true
		case v.IsNumeric():
			return 0, true
		default:
			return 0, false
		}
	}
	lo, lok := ordinal(l)
	ro, rok := ordinal(r)
	if !lok || !rok {
		return 0, false
	}
	switch {
	case lo == ro:
		return 0, true
	case lo > ro:
		return 1, true
	default:
		return -1, true
	}
}

// AsReal returns the value as a float64 (int and real only).
func (v Value) AsReal() float64 {
	if v.Kind == ValInt {
		return float64(v.Int)
	}
	return v.Real
}

// Eval attempts to evaluate n as a model-level constant. It returns ok=false
// for anything outside the supported subset (feature references, strings, null,
// unsupported operators, or arithmetic on infinity) — callers then skip the
// check, matching the pilot's model-level-evaluable gating.
func (m *Model) Eval(n ast.Node) (Value, bool) {
	return evalConst(n)
}

// EvalIn is Eval reading through the features n names, in scope, to the values
// they are bound to (`attribute one = 1;` then `[one]`).
func (m *Model) EvalIn(scope *symbols.Scope, n ast.Node) (Value, bool) {
	return m.evalIn(scope, n, nil)
}

func (m *Model) evalIn(scope *symbols.Scope, n ast.Node, seen map[*symbols.Symbol]bool) (Value, bool) {
	if m == nil || n == nil {
		return Value{}, false
	}
	switch e := n.(type) {
	case *ast.QualifiedName, *ast.FeatureReference, *ast.FeatureChainExpr:
		return m.evalFeatureIn(scope, n, seen)
	case *ast.OperatorExpr:
		return m.evalOperatorIn(scope, e, seen)
	default:
		return evalConst(n)
	}
}

// evalFeatureIn evaluates the value the feature ref names is bound to. seen
// guards a value that names itself.
func (m *Model) evalFeatureIn(scope *symbols.Scope, ref ast.Node, seen map[*symbols.Symbol]bool) (Value, bool) {
	if m.resolver == nil {
		return Value{}, false
	}
	sym, ok := m.resolver.ResolveTarget(scope, ref)
	if !ok || sym == nil || seen[sym] {
		return Value{}, false
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Value == nil {
		return Value{}, false
	}
	next := make(map[*symbols.Symbol]bool, len(seen)+1)
	for s := range seen {
		next[s] = true
	}
	next[sym] = true
	return m.evalIn(declScope(sym), usage.Value, next)
}

func (m *Model) evalOperatorIn(scope *symbols.Scope, e *ast.OperatorExpr, seen map[*symbols.Symbol]bool) (Value, bool) {
	switch e.Operator {
	case ast.OpNeg, ast.OpPos, ast.OpNot:
		if len(e.Operands) != 1 {
			return Value{}, false
		}
		v, ok := m.evalIn(scope, e.Operands[0], seen)
		if !ok {
			return Value{}, false
		}
		return EvalUnary(e.Operator, v)
	case ast.OpConditional:
		if len(e.Operands) != 3 {
			return Value{}, false
		}
		cond, ok := m.evalIn(scope, e.Operands[0], seen)
		if !ok || cond.Kind != ValBool {
			return Value{}, false
		}
		if cond.Bool {
			return m.evalIn(scope, e.Operands[1], seen)
		}
		return m.evalIn(scope, e.Operands[2], seen)
	default:
		if len(e.Operands) != 2 {
			return Value{}, false
		}
		l, lok := m.evalIn(scope, e.Operands[0], seen)
		r, rok := m.evalIn(scope, e.Operands[1], seen)
		if !lok || !rok {
			return Value{}, false
		}
		return EvalBinary(e.Operator, l, r)
	}
}

// declScope is the scope a declaration's own references resolve in.
func declScope(sym *symbols.Symbol) *symbols.Scope {
	if sym == nil {
		return nil
	}
	if sym.OwnerScope != nil {
		return sym.OwnerScope
	}
	return sym.Scope
}

func evalConst(n ast.Node) (Value, bool) {
	switch e := n.(type) {
	case *ast.LiteralInteger:
		i, err := strconv.ParseInt(e.Value, 10, 64)
		if err != nil {
			return Value{}, false
		}
		return Value{Kind: ValInt, Int: i}, true
	case *ast.LiteralReal:
		f, err := ParseReal(e.Value)
		if err != nil {
			return Value{}, false
		}
		return Value{Kind: ValReal, Real: f}, true
	case *ast.LiteralBool:
		return Value{Kind: ValBool, Bool: e.Value}, true
	case *ast.LiteralInfinity:
		return Value{Kind: ValInfinity}, true
	case *ast.OperatorExpr:
		return evalOperator(e)
	default:
		return Value{}, false
	}
}

func evalOperator(e *ast.OperatorExpr) (Value, bool) {
	switch e.Operator {
	case ast.OpNeg, ast.OpPos, ast.OpNot:
		if len(e.Operands) != 1 {
			return Value{}, false
		}
		return evalUnary(e.Operator, e.Operands[0])
	case ast.OpConditional:
		if len(e.Operands) != 3 {
			return Value{}, false
		}
		cond, ok := evalConst(e.Operands[0])
		if !ok || cond.Kind != ValBool {
			return Value{}, false
		}
		if cond.Bool {
			return evalConst(e.Operands[1])
		}
		return evalConst(e.Operands[2])
	default:
		if len(e.Operands) != 2 {
			return Value{}, false
		}
		return evalBinary(e.Operator, e.Operands[0], e.Operands[1])
	}
}

// EvalUnary evaluates a unary operator on a constant value.
// Returns (result, true) if successful, (zero, false) otherwise.
func EvalUnary(op ast.OperatorKind, v Value) (Value, bool) {
	switch op {
	case ast.OpNeg:
		if v.Kind == ValInt {
			// The least Integer has no negation within the range.
			if v.Int == math.MinInt64 {
				return Value{}, false
			}
			return Value{Kind: ValInt, Int: -v.Int}, true
		}
		if v.Kind == ValReal {
			return Value{Kind: ValReal, Real: -v.Real}, true
		}
	case ast.OpPos:
		if v.IsNumeric() {
			return v, true
		}
	case ast.OpNot:
		if v.Kind == ValBool {
			return Value{Kind: ValBool, Bool: !v.Bool}, true
		}
	}
	return Value{}, false
}

func evalUnary(op ast.OperatorKind, operand ast.Node) (Value, bool) {
	v, ok := evalConst(operand)
	if !ok {
		return Value{}, false
	}
	return EvalUnary(op, v)
}

func evalBinary(op ast.OperatorKind, lhs, rhs ast.Node) (Value, bool) {
	l, ok := evalConst(lhs)
	if !ok {
		return Value{}, false
	}
	r, ok := evalConst(rhs)
	if !ok {
		return Value{}, false
	}

	switch op {
	case ast.OpAnd, ast.OpConditionalAnd, ast.OpOr, ast.OpConditionalOr, ast.OpXor, ast.OpImplies:
		if l.Kind != ValBool || r.Kind != ValBool {
			return Value{}, false
		}
		return evalBoolOp(op, l.Bool, r.Bool), true
	case ast.OpEq, ast.OpNeq:
		return evalEquality(op, l, r)
	case ast.OpLt, ast.OpGt, ast.OpLe, ast.OpGe:
		return evalComparison(op, l, r)
	case ast.OpAdd, ast.OpSub, ast.OpMul, ast.OpDiv, ast.OpMod, ast.OpPow:
		return evalArithmetic(op, l, r)
	}
	return Value{}, false
}

// EvalBinary evaluates a binary operator on two constant values.
// Returns (result, true) if successful, (zero, false) otherwise.
func EvalBinary(op ast.OperatorKind, l, r Value) (Value, bool) {
	switch op {
	case ast.OpAnd, ast.OpConditionalAnd, ast.OpOr, ast.OpConditionalOr, ast.OpXor, ast.OpImplies:
		if l.Kind != ValBool || r.Kind != ValBool {
			return Value{}, false
		}
		return evalBoolOp(op, l.Bool, r.Bool), true
	case ast.OpEq, ast.OpNeq:
		return evalEquality(op, l, r)
	case ast.OpLt, ast.OpGt, ast.OpLe, ast.OpGe:
		return evalComparison(op, l, r)
	case ast.OpAdd, ast.OpSub, ast.OpMul, ast.OpDiv, ast.OpMod, ast.OpPow:
		return evalArithmetic(op, l, r)
	}
	return Value{}, false
}

func evalBoolOp(op ast.OperatorKind, a, b bool) Value {
	var res bool
	switch op {
	case ast.OpAnd, ast.OpConditionalAnd:
		res = a && b
	case ast.OpOr, ast.OpConditionalOr:
		res = a || b
	case ast.OpXor:
		res = a != b
	case ast.OpImplies:
		res = !a || b
	}
	return Value{Kind: ValBool, Bool: res}
}

func evalEquality(op ast.OperatorKind, l, r Value) (Value, bool) {
	var eq bool
	switch {
	case l.Kind == ValBool && r.Kind == ValBool:
		eq = l.Bool == r.Bool
	case l.IsUnbounded() || r.IsUnbounded():
		order, ok := UnboundedOrder(l, r)
		if !ok {
			return Value{}, false
		}
		eq = order == 0
	case l.IsNumeric() && r.IsNumeric():
		eq = l.AsReal() == r.AsReal()
	default:
		return Value{}, false
	}
	if op == ast.OpNeq {
		eq = !eq
	}
	return Value{Kind: ValBool, Bool: eq}, true
}

func evalComparison(op ast.OperatorKind, l, r Value) (Value, bool) {
	if l.IsUnbounded() || r.IsUnbounded() {
		order, ok := UnboundedOrder(l, r)
		if !ok {
			return Value{}, false
		}
		res, ok := OrderSatisfies(op, order)
		if !ok {
			return Value{}, false
		}
		return Value{Kind: ValBool, Bool: res}, true
	}
	if !l.IsNumeric() || !r.IsNumeric() {
		return Value{}, false
	}
	lf, rf := l.AsReal(), r.AsReal()
	var res bool
	switch op {
	case ast.OpLt:
		res = lf < rf
	case ast.OpGt:
		res = lf > rf
	case ast.OpLe:
		res = lf <= rf
	case ast.OpGe:
		res = lf >= rf
	}
	return Value{Kind: ValBool, Bool: res}, true
}

// OrderSatisfies applies an ordering operator to a comparison result of -1, 0
// or 1. ok is false for an operator that is no ordering.
func OrderSatisfies(op ast.OperatorKind, order int) (res, ok bool) {
	switch op {
	case ast.OpLt:
		return order < 0, true
	case ast.OpLe:
		return order <= 0, true
	case ast.OpGt:
		return order > 0, true
	case ast.OpGe:
		return order >= 0, true
	default:
		return false, false
	}
}

func evalArithmetic(op ast.OperatorKind, l, r Value) (Value, bool) {
	if !l.IsNumeric() || !r.IsNumeric() {
		return Value{}, false
	}
	if op == ast.OpPow {
		v, err := Pow(l, r)
		if err != nil {
			return Value{}, false
		}
		return v, true
	}
	// Integer arithmetic when both operands are integers (except division,
	// which may be fractional — keep it real to avoid silent truncation).
	if l.Kind == ValInt && r.Kind == ValInt {
		if op == ast.OpDiv {
			q, ok := IntQuotient(l.Int, r.Int)
			if !ok {
				return Value{}, false
			}
			return Value{Kind: ValReal, Real: q}, true
		}
		return evalIntArith(op, l.Int, r.Int)
	}
	return evalRealArith(op, l.AsReal(), r.AsReal())
}

// evalIntArith folds Integer arithmetic, declining a result outside the
// Integer range: the run time reports it, so nothing folds to a wrapped value.
func evalIntArith(op ast.OperatorKind, a, b int64) (Value, bool) {
	switch op {
	case ast.OpAdd, ast.OpSub, ast.OpMul:
		res, ok := IntArith(op, a, b)
		if !ok {
			return Value{}, false
		}
		return Value{Kind: ValInt, Int: res}, true
	case ast.OpMod:
		if b == 0 {
			return Value{}, false
		}
		return Value{Kind: ValInt, Int: a % b}, true
	}
	return Value{}, false
}

// evalRealArith folds Real arithmetic, declining a result that is not a finite
// Real so nothing folds to an infinity: the run time reports it.
func evalRealArith(op ast.OperatorKind, a, b float64) (Value, bool) {
	res, ok := RealArith(op, a, b)
	if !ok || math.IsInf(res, 0) || math.IsNaN(res) {
		return Value{}, false
	}
	return Value{Kind: ValReal, Real: res}, true
}

// IntQuotient is the exact rational quotient of two Integers rounded once to
// the nearest float64, shared by the folder and the runtime; ok=false on a
// zero divisor. Rounding each operand first would move quotients of operands
// beyond 2^53, where int64 loses exactness in float64.
func IntQuotient(a, b int64) (float64, bool) {
	if b == 0 {
		return 0, false
	}
	q, _ := new(big.Rat).SetFrac64(a, b).Float64()
	return q, true
}

// RealArith is Real addition, subtraction, multiplication and division,
// reporting ok=false for an operator it does not define or a division by zero.
func RealArith(op ast.OperatorKind, a, b float64) (float64, bool) {
	switch op {
	case ast.OpAdd:
		return a + b, true
	case ast.OpSub:
		return a - b, true
	case ast.OpMul:
		return a * b, true
	case ast.OpDiv:
		if b == 0 {
			return 0, false
		}
		return a / b, true
	}
	return 0, false
}

// Pow evaluates l ** r (equivalently l ^ r) — the single implementation the
// constant folder and the runtime share, so a folded and an evaluated
// exponentiation agree. Integer operands with a non-negative exponent give an
// Integer, as IntegerFunctions::'**' declares; every other numeric combination
// gives a Real, as RealFunctions::'**' does. A result that is not a finite value
// of that kind is an error rather than a NaN, an infinity, or a wrapped integer:
// the folder declines on it, the runtime reports it.
func Pow(l, r Value) (Value, error) {
	if !l.IsNumeric() || !r.IsNumeric() {
		return Value{}, fmt.Errorf("%w: ** requires numeric operands", ErrArithmeticDomain)
	}

	if l.Kind == ValInt && r.Kind == ValInt && r.Int >= 0 {
		res, ok := intPow(l.Int, r.Int)
		if !ok {
			return Value{}, fmt.Errorf("%w: %d ** %d exceeds the Integer range", ErrArithmeticOverflow, l.Int, r.Int)
		}
		return Value{Kind: ValInt, Int: res}, nil
	}

	base, exp := l.AsReal(), r.AsReal()
	switch {
	case base == 0 && exp < 0:
		return Value{}, fmt.Errorf("%w: 0 ** %v is undefined (negative exponent)", ErrArithmeticDomain, exp)
	case base < 0 && exp != math.Trunc(exp):
		return Value{}, fmt.Errorf("%w: %v ** %v is not a Real (negative base, fractional exponent)", ErrArithmeticDomain, base, exp)
	}
	res := math.Pow(base, exp)
	if math.IsNaN(res) || math.IsInf(res, 0) {
		return Value{}, fmt.Errorf("%w: %v ** %v is not a finite Real", ErrArithmeticOverflow, base, exp)
	}
	return Value{Kind: ValReal, Real: res}, nil
}

// intPow computes a**n for n >= 0 by repeated squaring, reporting ok=false when
// the result leaves the int64 range rather than wrapping.
func intPow(a, n int64) (int64, bool) {
	res := int64(1)
	for n > 0 {
		if n&1 == 1 {
			var ok bool
			if res, ok = mulInt(res, a); !ok {
				return 0, false
			}
		}
		if n >>= 1; n == 0 {
			break
		}
		var ok bool
		if a, ok = mulInt(a, a); !ok {
			return 0, false
		}
	}
	return res, true
}

// IntArith is Integer addition, subtraction and multiplication, shared by the
// folder, the operators and the library functions, reporting ok=false on overflow.
func IntArith(op ast.OperatorKind, a, b int64) (int64, bool) {
	switch op {
	case ast.OpAdd:
		res := a + b
		return res, (b <= 0 || res > a) && (b >= 0 || res < a)
	case ast.OpSub:
		res := a - b
		return res, (b >= 0 || res > a) && (b <= 0 || res < a)
	case ast.OpMul:
		return mulInt(a, b)
	}
	return 0, false
}

// mulInt multiplies two int64 values, reporting ok=false on overflow.
func mulInt(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	// The least int64 negated is not an int64, and the division check below
	// cannot see that case because the quotient equals the dividend.
	if (a == math.MinInt64 && b == -1) || (b == math.MinInt64 && a == -1) {
		return 0, false
	}
	res := a * b
	if res/b != a {
		return 0, false
	}
	return res, true
}
