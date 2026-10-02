package semantics

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

var (
	// ErrArithmeticDomain reports operands an operation is not defined for, such
	// as a negative base raised to a fractional exponent.
	ErrArithmeticDomain = errors.New("arithmetic domain error")

	// ErrArithmeticOverflow reports a result outside the range of the kind it
	// would have: a non-finite Real.
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
type ValueKind uint8

const (
	ValInvalid ValueKind = iota
	ValInt
	ValReal
	ValBool
	ValInfinity // the `*` bound / unbounded value
	ValRational // an exact Rational, held in lowest terms
)

// StringTypeName names the scalar type of a string value, as a sent signal's type.
const StringTypeName = "String"

// ScalarTypeName names the scalar type of a constant of kind, as a sent
// signal's type, or "" for a kind no sent value has.
func ScalarTypeName(kind ValueKind) string {
	switch kind {
	case ValInt:
		return "Integer"
	case ValReal:
		return "Real"
	case ValRational:
		return "Rational"
	case ValBool:
		return "Boolean"
	}
	return ""
}

// SentScalarTypes names the scalar types a send of node's value may send, as
// the runtime names them: one when node's scalar type is known (a Rational also
// as a Real), Integer and Real for a number of unknown kind, and none when
// node's value is not a scalar.
func (m *Model) SentScalarTypes(scope *symbols.Scope, node ast.Node) []string {
	if _, ok := node.(*ast.LiteralString); ok {
		return []string{StringTypeName}
	}
	if value, ok := m.Eval(node); ok {
		// A Rational is a Real (ScalarValues::Rational specializes Real).
		if value.Kind == ValRational {
			return []string{ScalarTypeName(ValRational), ScalarTypeName(ValReal)}
		}
		if name := ScalarTypeName(value.Kind); name != "" {
			return []string{name}
		}
		return nil
	}
	integerName, realName := ScalarTypeName(ValInt), ScalarTypeName(ValReal)
	for _, scalar := range []struct{ fqn, name string }{
		{fqnString, StringTypeName},
		{FQNBoolean, ScalarTypeName(ValBool)},
		{fqnInteger, integerName},
		{fqnReal, realName},
	} {
		if c := m.ExprConformsToLibrary(scope, node, scalar.fqn); c.Known && c.Holds {
			return []string{scalar.name}
		}
	}
	if c := m.ExprConformsToLibrary(scope, node, fqnNumericalValue); c.Known && c.Holds {
		return []string{integerName, realName}
	}
	return nil
}

// Value is a model-level-evaluated constant. Only the field selected by Kind is
// meaningful. This is a deliberately small subset: the constraint checks that
// need evaluation (multiplicity bounds, some guards) operate over integers,
// reals, booleans, and the infinity bound.
//
// An Integer is unbounded. One within int64 is held in Int; any other is held
// as an immutable big.Int, with Int zero, so each Integer has one
// representation. Read an Integer through Int64, BigInt or the Int functions
// rather than through Int, which is meaningful only when IsBigInt is false.
//
// A Rational is exact (rational.go): its numerator in Int and positive
// denominator in den when both fit int64, an immutable big.Rat otherwise. A
// Real is an IEEE 754 binary64.
type Value struct {
	Kind ValueKind
	Bool bool
	den  uint32
	Int  int64
	Real float64
	ext  *big.Rat
}

// IsNumeric reports whether the value is an Integer, a Rational or a Real.
func (v Value) IsNumeric() bool {
	return v.Kind == ValInt || v.Kind == ValReal || v.Kind == ValRational
}

// WholeNumber returns the value as an int64 when it is a whole number within
// that range: an integer, or a finite real with no fractional part (4 / 2 is 2.0).
func (v Value) WholeNumber() (int64, bool) {
	switch v.Kind {
	case ValInt:
		return v.Int64()
	case ValRational:
		if !v.RatIsWhole() {
			return 0, false
		}
		return v.RatNumer().Int64()
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

// AsReal returns the number as a float64. An Integer or a Rational rounds to
// the nearest float64, ties to even, and one beyond the float64 range to the
// infinity of its sign: the one conversion mixed Rational/Real arithmetic makes.
func (v Value) AsReal() float64 {
	switch v.Kind {
	case ValInt:
		return intToFloat(v)
	case ValRational:
		return ratToFloat(v)
	}
	return v.Real
}

// Eval attempts to evaluate n as a model-level constant. It returns ok=false
// for anything outside the supported subset (feature references, strings, null,
// unsupported operators, or arithmetic on infinity) — callers then skip the
// check, matching the pilot's model-level-evaluable gating.
func (m *Model) Eval(n ast.Node) (Value, bool) {
	return EvalConst(n)
}

// EvalWithin is Eval under an Integer size budget of maxBits rather than the
// default: it declines a fold any of whose Integer results would need more, so
// the run evaluating the expression reports it under the budget it runs with.
func (m *Model) EvalWithin(n ast.Node, maxBits int64) (Value, bool) {
	return evalConst(n, maxBits)
}

// EvalIn is Eval reading through the features n names, in scope, to the values
// they are bound to (`attribute one = 1;` then `[one]`).
func (m *Model) EvalIn(scope *symbols.Scope, n ast.Node) (Value, bool) {
	return m.evalIn(scope, n, nil)
}

// ReadsValuelessFeature reports whether evaluating n in scope reaches a feature
// with no value, directly or through the values of the features it reads.
func (m *Model) ReadsValuelessFeature(scope *symbols.Scope, n ast.Node) bool {
	if m == nil || n == nil || m.resolver == nil {
		return false
	}
	return m.readsValueless(scope, n, nil)
}

func (m *Model) readsValueless(scope *symbols.Scope, n ast.Node, seen map[*symbols.Symbol]bool) bool {
	if m == nil || n == nil || m.resolver == nil {
		return false
	}
	switch e := n.(type) {
	case *ast.QualifiedName, *ast.FeatureReference, *ast.FeatureChainExpr:
		sym, ok := m.resolver.ResolveTarget(scope, n)
		if !ok || sym == nil {
			return false
		}
		if target, aliasOK := m.resolver.ResolveAliasTarget(sym); aliasOK {
			sym = target
		}
		if sym == nil || seen[sym] || !sym.Kind.IsFeature() {
			return false
		}
		usage, ok := sym.Decl.(*ast.Usage)
		if !ok {
			return false
		}
		if usage.Value == nil {
			return true
		}
		next := make(map[*symbols.Symbol]bool, len(seen)+1)
		for s := range seen {
			next[s] = true
		}
		next[sym] = true
		return m.readsValueless(declScope(sym), usage.Value, next)
	case *ast.OperatorExpr:
		if e.Operator == ast.OpConditional && len(e.Operands) == 3 {
			cond, ok := m.evalIn(scope, e.Operands[0], seen)
			if ok && cond.Kind == ValBool {
				if cond.Bool {
					return m.readsValueless(scope, e.Operands[1], seen)
				}
				return m.readsValueless(scope, e.Operands[2], seen)
			}
		}
		for _, operand := range e.Operands {
			if m.readsValueless(scope, operand, seen) {
				return true
			}
		}
	}
	return false
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
		return EvalConst(n)
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

// EvalConst is Eval without a model: the value of an expression over literals
// and operators alone, which no feature's binding can change.
func EvalConst(n ast.Node) (Value, bool) {
	return evalConst(n, DefaultMaxIntegerBits)
}

func evalConst(n ast.Node, maxBits int64) (Value, bool) {
	switch e := n.(type) {
	case *ast.LiteralInteger:
		return ParseInteger(e.Value)
	case *ast.LiteralReal:
		v, err := ParseRational(e.Value, maxBits)
		if err != nil {
			return Value{}, false
		}
		return v, true
	case *ast.LiteralBool:
		return Value{Kind: ValBool, Bool: e.Value}, true
	case *ast.LiteralInfinity:
		return Value{Kind: ValInfinity}, true
	case *ast.OperatorExpr:
		return evalOperator(e, maxBits)
	default:
		return Value{}, false
	}
}

func evalOperator(e *ast.OperatorExpr, maxBits int64) (Value, bool) {
	switch e.Operator {
	case ast.OpNeg, ast.OpPos, ast.OpNot:
		if len(e.Operands) != 1 {
			return Value{}, false
		}
		return evalUnary(e.Operator, e.Operands[0], maxBits)
	case ast.OpConditional:
		if len(e.Operands) != 3 {
			return Value{}, false
		}
		cond, ok := evalConst(e.Operands[0], maxBits)
		if !ok || cond.Kind != ValBool {
			return Value{}, false
		}
		if cond.Bool {
			return evalConst(e.Operands[1], maxBits)
		}
		return evalConst(e.Operands[2], maxBits)
	default:
		if len(e.Operands) != 2 {
			return Value{}, false
		}
		return evalBinary(e.Operator, e.Operands[0], e.Operands[1], maxBits)
	}
}

// EvalUnary evaluates a unary operator on a constant value.
// Returns (result, true) if successful, (zero, false) otherwise.
func EvalUnary(op ast.OperatorKind, v Value) (Value, bool) {
	switch op {
	case ast.OpNeg:
		if v.Kind == ValInt {
			return IntNeg(v), true
		}
		if v.Kind == ValRational {
			return RatNeg(v), true
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

func evalUnary(op ast.OperatorKind, operand ast.Node, maxBits int64) (Value, bool) {
	v, ok := evalConst(operand, maxBits)
	if !ok {
		return Value{}, false
	}
	return EvalUnary(op, v)
}

func evalBinary(op ast.OperatorKind, lhs, rhs ast.Node, maxBits int64) (Value, bool) {
	l, ok := evalConst(lhs, maxBits)
	if !ok {
		return Value{}, false
	}
	r, ok := evalConst(rhs, maxBits)
	if !ok {
		return Value{}, false
	}
	return evalBinaryWithin(op, l, r, maxBits)
}

// EvalBinary evaluates a binary operator on two constant values.
// Returns (result, true) if successful, (zero, false) otherwise.
func EvalBinary(op ast.OperatorKind, l, r Value) (Value, bool) {
	return evalBinaryWithin(op, l, r, DefaultMaxIntegerBits)
}

func evalBinaryWithin(op ast.OperatorKind, l, r Value, maxBits int64) (Value, bool) {
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
		return evalArithmetic(op, l, r, maxBits)
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
	case l.Kind == ValInt && r.Kind == ValInt:
		eq = CompareInt(l, r) == 0
	case l.IsExact() && r.IsExact():
		eq = CompareRat(l, r) == 0
	case l.IsExact() && r.Kind == ValReal:
		eq = !math.IsNaN(r.Real) && CompareExactReal(l, r.Real) == 0
	case l.Kind == ValReal && r.IsExact():
		eq = !math.IsNaN(l.Real) && CompareExactReal(r, l.Real) == 0
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
	if l.Kind == ValInt && r.Kind == ValInt {
		res, _ := OrderSatisfies(op, CompareInt(l, r))
		return Value{Kind: ValBool, Bool: res}, true
	}
	if l.IsExact() && r.IsExact() {
		res, _ := OrderSatisfies(op, CompareRat(l, r))
		return Value{Kind: ValBool, Bool: res}, true
	}
	if l.IsExact() && r.Kind == ValReal && !math.IsNaN(r.Real) {
		res, _ := OrderSatisfies(op, CompareExactReal(l, r.Real))
		return Value{Kind: ValBool, Bool: res}, true
	}
	if l.Kind == ValReal && r.IsExact() && !math.IsNaN(l.Real) {
		res, _ := OrderSatisfies(op, -CompareExactReal(r, l.Real))
		return Value{Kind: ValBool, Bool: res}, true
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

func evalArithmetic(op ast.OperatorKind, l, r Value, maxBits int64) (Value, bool) {
	if !l.IsNumeric() || !r.IsNumeric() {
		return Value{}, false
	}
	if op == ast.OpPow {
		v, err := Pow(l, r, maxBits)
		if err != nil {
			return Value{}, false
		}
		return v, true
	}
	// Integer arithmetic stays Integer except division, which IntegerFunctions
	// declares Rational; exact operands give an exact Rational; a Real operand
	// makes the operation binary64.
	if l.Kind == ValInt && r.Kind == ValInt && op != ast.OpDiv {
		return evalIntArith(op, l, r, maxBits)
	}
	if l.IsExact() && r.IsExact() {
		res, err := RatArith(op, l, r, maxBits)
		if err != nil {
			return Value{}, false
		}
		return res, true
	}
	return evalRealArith(op, l.AsReal(), r.AsReal())
}

// evalIntArith folds Integer arithmetic, declining a result beyond maxBits: the
// run time reports it under the budget it runs with.
func evalIntArith(op ast.OperatorKind, a, b Value, maxBits int64) (Value, bool) {
	switch op {
	case ast.OpAdd, ast.OpSub, ast.OpMul:
		res, err := IntArith(op, a, b, maxBits)
		if err != nil {
			return Value{}, false
		}
		return res, true
	case ast.OpMod:
		return IntRem(a, b)
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
// Integer, as IntegerFunctions::'**' declares, refused with ErrIntegerSizeLimit
// when it would need more than maxBits; an exact base with any other whole
// exponent gives the exact Rational RationalFunctions::'**' declares, refused
// with ErrRationalSizeLimit; every other numeric combination gives a Real, as
// RealFunctions::'**' does. A Real result that is not finite is an
// error rather than a NaN or an infinity: the folder declines on it, the
// runtime reports it.
func Pow(l, r Value, maxBits int64) (Value, error) {
	if !l.IsNumeric() || !r.IsNumeric() {
		return Value{}, fmt.Errorf("%w: ** requires numeric operands", ErrArithmeticDomain)
	}

	if l.Kind == ValInt && r.Kind == ValInt && r.IntSign() >= 0 {
		return IntPow(l, r, maxBits)
	}
	// RationalFunctions::'**' takes an Integer exponent; a Rational one is RealFunctions'.
	if l.IsExact() && r.Kind == ValInt {
		return RatPow(l, r, maxBits)
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
