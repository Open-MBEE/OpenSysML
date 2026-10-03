package codegen

import (
	"fmt"
	"math"
	"math/big"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// exactness is how the interpreter holds a numeric expression's value.
type exactness int

const (
	// binary64 is a Real, held as binary64.
	binary64 exactness = iota
	// exactWhole is an exact whole number of magnitude at most 2^53.
	exactWhole
	// exactDouble is an exact Integer or Rational that binary64 holds exactly.
	exactDouble
	// exactRational is an exact Integer or Rational binary64 may not hold.
	exactRational
)

const maxExactDouble = 1 << 53

const negativePower = "'**' of an exact Rational by an Integer exponent (compiled code holds Rationals as binary64)"

// exactness classifies x. Compiled code holds every Rational as binary64, so
// it compiles exact arithmetic only where one rounding gives the exact result.
func (fc *funcCompiler) exactness(x Expr) exactness { return fc.classify(x, true) }

// classify is exactness; reached is whether exactWiden can still guard x's
// Integer leaves and whole results, which a temporary or element has lost.
func (fc *funcCompiler) classify(x Expr, reached bool) exactness {
	switch x := x.(type) {
	case IntLit:
		if x.Big == nil && x.Value >= -maxExactDouble && x.Value <= maxExactDouble {
			return exactWhole
		}
		return exactRational
	case RealLit:
		if x.Rat == nil {
			return binary64
		}
		if f, exact := x.Rat.Float64(); exact {
			if x.Rat.IsInt() && math.Abs(f) <= maxExactDouble {
				return exactWhole
			}
			return exactDouble
		}
		return exactRational
	case Var:
		if e, ok := fc.exactTemps[x.Name]; ok {
			return e
		}
	case ToReal:
		if !reached && !x.Exact && x.X.Type().Elem() == TypeInt {
			return exactRational
		}
		return fc.classify(x.X, reached)
	case Unary:
		return fc.classify(x.X, reached)
	case Binary:
		if x.T.Elem() == TypeReal && isArithmetic(x.Op) {
			if fc.classify(x.L, reached) == binary64 || fc.classify(x.R, reached) == binary64 {
				return binary64
			}
			if x.Whole && (reached || x.Guard) {
				return exactWhole
			}
			return exactRational
		}
	case Cond:
		return fc.classifyBranches(reached, x.Then, x.Else)
	case LibCall:
		switch x.Op {
		case LibAbsReal, LibMaxReal, LibMinReal:
			e := fc.classify(x.Args[0].Value, reached)
			for _, a := range x.Args[1:] {
				e = join(e, fc.classify(a.Value, reached))
			}
			return e
		}
	case ToMany:
		return fc.classify(x.X, reached)
	case Steps:
		return fc.classify(x.X, reached)
	case ToNum:
		return fc.classify(x.X, reached)
	case NumSplit:
		return fc.classifyBranches(reached, x.Int, x.Real)
	case ToOne:
		return fc.classify(x.X, reached)
	case Checked:
		return fc.classify(x.X, reached)
	case Framed:
		return fc.classify(x.X, reached)
	case Let:
		return fc.classify(x.In, reached)
	case Coalesce:
		return join(fc.classify(x.L, false), fc.classify(x.R, false))
	case SeqLit:
		if len(x.Elems) == 0 {
			return binary64
		}
		e := fc.classify(x.Elems[0], false)
		for _, el := range x.Elems[1:] {
			e = join(e, fc.classify(el, false))
		}
		return e
	case Index:
		return fc.classify(x.Seq, false)
	}
	if x.Type().Elem() == TypeInt {
		return exactWhole
	}
	return binary64
}

// classifyBranches is the class of a value one of branches gives, a refusal
// giving none.
func (fc *funcCompiler) classifyBranches(reached bool, branches ...Expr) exactness {
	e, seen := binary64, false
	for _, x := range branches {
		if _, refused := x.(Refusal); refused {
			continue
		}
		if c := fc.classify(x, reached); seen {
			e = join(e, c)
		} else {
			e, seen = c, true
		}
	}
	return e
}

// join is the class of a value that is either of class a or of class b: a Real
// or an exact number binary64 holds is held exactly, though not known whole.
func join(a, b exactness) exactness {
	if (a == binary64) != (b == binary64) && max(a, b) != exactRational {
		return exactDouble
	}
	return max(a, b)
}

// heldExactly is whether binary64 holds every value of class e exactly.
func heldExactly(e exactness) bool { return e == exactWhole || e == exactDouble }

func isArithmetic(op ast.OperatorKind) bool {
	switch op {
	case ast.OpAdd, ast.OpSub, ast.OpMul, ast.OpDiv, ast.OpMod, ast.OpPow:
		return true
	}
	return false
}

// constRat is the exact value of a constant numeric expression.
func constRat(x Expr) (*big.Rat, bool) {
	switch x := bare(x).(type) {
	case RealLit:
		return x.Rat, x.Rat != nil
	case IntLit:
		if x.Big != nil {
			return new(big.Rat).SetInt(x.Big), true
		}
		return new(big.Rat).SetInt64(x.Value), true
	case ToReal:
		if !x.X.Type().Many() {
			return constRat(x.X)
		}
	}
	return nil, false
}

// exactLit is the literal of the exact Rational r, if binary64 has a finite
// nonzero-for-nonzero value for it.
func exactLit(r *big.Rat) (RealLit, bool) {
	f, _ := r.Float64()
	if math.IsInf(f, 0) || (f == 0 && r.Sign() != 0) {
		return RealLit{}, false
	}
	return RealLit{Value: f + 0, Rat: r}, true
}

// rationalArithmetic checks the Real-typed arithmetic b. With a Real operand
// it is binary64 arithmetic; over exact operands only constants (folded) and a
// single operation over values binary64 holds exactly (one correct rounding)
// agree with the interpreter's exact Rational result.
func (fc *funcCompiler) rationalArithmetic(b Binary) (Expr, error) {
	x, why := fc.exactArith(b)
	if why != "" {
		return nil, fc.unsupported(why)
	}
	return x, nil
}

// exactArith is rationalArithmetic, giving the reason it refuses b.
func (fc *funcCompiler) exactArith(b Binary) (Expr, string) {
	if x, ok := fc.overSplit(b, fc.exactArith); ok {
		return x, ""
	}
	l, r := fc.exactness(b.L), fc.exactness(b.R)
	if l == binary64 || r == binary64 {
		return b, ""
	}
	if b.Op == ast.OpPow {
		if !isWidenedInt(bare(b.R)) {
			return b, ""
		}
		if a, okA := constRat(b.L); okA {
			if n, okN := bare(bare(b.R).(ToReal).X).(IntLit); okN && n.Big == nil {
				if v, err := semantics.RatPow(semantics.RatValue(a), semantics.IntValue(n.Value), semantics.DefaultMaxIntegerBits); err == nil {
					if lit, ok := exactLit(v.Rat()); ok {
						return lit, ""
					}
				}
			}
		}
		return nil, negativePower
	}
	if a, okA := constRat(b.L); okA {
		if c, okC := constRat(b.R); okC && b.Op != ast.OpMod {
			if v, err := semantics.RatArith(b.Op, semantics.RatValue(a), semantics.RatValue(c), semantics.DefaultMaxIntegerBits); err == nil {
				if lit, ok := exactLit(v.Rat()); ok {
					return lit, ""
				}
			}
		}
	}
	if b.Op == ast.OpDiv && bare(b.L).Type() == TypeInt {
		return b, ""
	}
	if heldExactly(l) && heldExactly(r) {
		if b.Op == ast.OpMul && (scalesExactly(b.L) || scalesExactly(b.R)) {
			// Scaling by 2^k, k >= 0, commutes with rounding, so neither operand needs a guard.
		} else {
			b.L, b.R = exactWiden(b.L), exactWiden(b.R)
		}
		b.Exact = true
		b.Whole = l == exactWhole && r == exactWhole && (b.Op == ast.OpAdd || b.Op == ast.OpSub || b.Op == ast.OpMul || b.Op == ast.OpMod)
		return b, ""
	}
	return nil, fmt.Sprintf("exact Rational arithmetic '%s' over a value binary64 does not hold exactly (compiled code holds Rationals as binary64)", b.Op)
}

// scalesExactly is whether x is a constant 2^k, k >= 0.
func scalesExactly(x Expr) bool {
	r, ok := constRat(x)
	if !ok || !r.IsInt() || r.Sign() <= 0 {
		return false
	}
	n := r.Num()
	return n.BitLen()-1 == int(n.TrailingZeroBits()) // #nosec G115 -- a trailing-zero count fits
}

// exactWiden guards x's Integer leaves and whole results, so that binary64
// holds them exactly or the program fails.
func exactWiden(x Expr) Expr {
	switch w := x.(type) {
	case Steps:
		w.X = exactWiden(w.X)
		return w
	case ToReal:
		if w.X.Type() == TypeInt {
			w.Exact = true
		}
		return w
	case Binary:
		if w.Whole {
			w.Guard = true
		}
		return w
	case Unary:
		if w.Exact {
			w.X = exactWiden(w.X)
		}
		return w
	case Cond:
		w.Then, w.Else = exactWiden(w.Then), exactWiden(w.Else)
		return w
	case ToOne:
		w.X = exactWiden(w.X)
		return w
	case Checked:
		w.X = exactWiden(w.X)
		return w
	case Framed:
		w.X = exactWiden(w.X)
		return w
	case Let:
		w.In = exactWiden(w.In)
		return w
	case NumSplit:
		w.Int, w.Real = exactWiden(w.Int), exactWiden(w.Real)
		return w
	case LibCall:
		switch w.Op {
		case LibAbsReal, LibMaxReal, LibMinReal:
			args := make([]Arg, len(w.Args))
			for i, a := range w.Args {
				args[i] = Arg{Param: a.Param, Value: exactWiden(a.Value)}
			}
			w.Args = args
		}
		return w
	}
	return x
}

// rationalComparison checks the comparison or equality b over Real operands.
// A Real against a Rational literal compares with the literal's nearest binary64.
func (fc *funcCompiler) rationalComparison(b Binary) (Expr, error) {
	x, why := fc.exactCompare(b)
	if why != "" {
		return nil, fc.unsupported(why)
	}
	return x, nil
}

// exactCompare is rationalComparison, giving the reason it refuses b.
func (fc *funcCompiler) exactCompare(b Binary) (Expr, string) {
	if b.L.Type() != TypeReal || b.R.Type() != TypeReal {
		return b, ""
	}
	if x, ok := fc.overSplit(b, fc.exactCompare); ok {
		return x, ""
	}
	l, r := fc.exactness(b.L), fc.exactness(b.R)
	if l != exactRational && r != exactRational {
		if l != binary64 {
			b.L = exactWiden(b.L)
		}
		if r != binary64 {
			b.R = exactWiden(b.R)
		}
		return b, ""
	}
	if m, x := stepped(b.L); r == exactWhole {
		if q, ok := intQuotient(x); ok {
			b.L, b.R = q, exactWiden(b.R)
			return restep(m, b), ""
		}
	}
	if m, x := stepped(b.R); l == exactWhole && (m == 0 || pure(b.L)) {
		if q, ok := intQuotient(x); ok {
			b.L, b.R = exactWiden(b.L), q
			return restep(m, b), ""
		}
	}
	if a, okA := constRat(b.L); okA {
		if c, okC := constRat(b.R); okC {
			return BoolLit{Value: compares(b.Op, a.Cmp(c))}, ""
		}
	}
	if c, ok := constRat(b.R); ok {
		if m, x := leading(b.L); isWidenedInt(x) {
			i, _ := widenedInt(x)
			k, _ := leading(b.R)
			return restep(m, fc.intAgainst(b.Op, i, k, c)), ""
		}
	}
	if c, ok := constRat(b.L); ok {
		if m, x := leading(b.R); isWidenedInt(x) {
			i, _ := widenedInt(x)
			k, _ := leading(b.L)
			return restep(k+m, fc.intAgainst(flipped(b.Op), i, 0, c)), ""
		}
	}
	if c, ok := constRat(b.R); ok && l == binary64 {
		k, _ := leading(b.R)
		return againstNearest(b.Op, b.L, k, c), ""
	}
	if c, ok := constRat(b.L); ok && r == binary64 {
		m, _ := leading(b.L)
		return restep(m, againstNearest(flipped(b.Op), b.R, 0, c)), ""
	}
	return nil, fmt.Sprintf("'%s' of an exact Rational binary64 does not hold exactly (compiled code holds Rationals as binary64)", b.Op)
}

// intQuotient is the Integer quotient x marked to be compared exactly.
func intQuotient(x Expr) (Binary, bool) {
	q, ok := x.(Binary)
	if !ok || q.Op != ast.OpDiv || q.L.Type() != TypeInt {
		return Binary{}, false
	}
	q.Exact = true
	return q, true
}

// stepped is x past the steps it spends first.
func stepped(x Expr) (int64, Expr) {
	var m int64
	for {
		s, ok := x.(Steps)
		if !ok {
			return m, x
		}
		m, x = m+s.N, s.X
	}
}

func compares(op ast.OperatorKind, c int) bool {
	switch op {
	case ast.OpLt:
		return c < 0
	case ast.OpLe:
		return c <= 0
	case ast.OpGt:
		return c > 0
	case ast.OpGe:
		return c >= 0
	case ast.OpEq:
		return c == 0
	}
	return c != 0
}

func flipped(op ast.OperatorKind) ast.OperatorKind {
	switch op {
	case ast.OpLt:
		return ast.OpGt
	case ast.OpLe:
		return ast.OpGe
	case ast.OpGt:
		return ast.OpLt
	case ast.OpGe:
		return ast.OpLe
	}
	return op
}

// againstNearest is `x op r` at Real precision: x against the binary64 nearest
// r, which is finite since a literal or fold past the range is refused; r's
// literal spends k steps after x.
func againstNearest(op ast.OperatorKind, x Expr, k int64, r *big.Rat) Binary {
	d, _ := r.Float64()
	return Binary{Op: op, L: x, R: restep(k, RealLit{Value: d}), T: TypeBool}
}

// intAgainst is `i op r` for the Integer i and exact constant r, compared as
// Integers; r's literal spends k steps after i.
func (fc *funcCompiler) intAgainst(op ast.OperatorKind, i Expr, k int64, r *big.Rat) Expr {
	against := func(op ast.OperatorKind, n *big.Int) Expr {
		return Binary{Op: op, L: i, R: restep(k, intLit(semantics.BigIntValue(n))), T: TypeBool}
	}
	if r.IsInt() {
		return against(op, r.Num())
	}
	floor := new(big.Int).Div(r.Num(), r.Denom())
	switch op {
	case ast.OpLt, ast.OpLe:
		return against(ast.OpLe, floor)
	case ast.OpGt, ast.OpGe:
		return against(ast.OpGt, floor)
	}
	// No Integer equals a fraction.
	_, lets := fc.hoist(i, nil)
	x := restep(k, BoolLit{Value: op == ast.OpNeq})
	for j := len(lets) - 1; j >= 0; j-- {
		x = Let{Name: lets[j].Name, Value: lets[j].Value, In: x}
	}
	return x
}

// restep is x spending m steps first.
func restep(m int64, x Expr) Expr {
	if m == 0 {
		return x
	}
	return Steps{N: m, X: x}
}

// overSplit applies exact to b over each branch of a number split operand
// whose class is exact, when the other operand is exact too. A right operand is
// evaluated in each branch, after the left, and a left one before the split.
func (fc *funcCompiler) overSplit(b Binary, exact func(Binary) (Expr, string)) (Expr, bool) {
	if fc.exactness(b.L) == binary64 || fc.exactness(b.R) == binary64 {
		return nil, false
	}
	if hasSplit(b.L) {
		return mapSplit(b.L, b.T, func(v Expr) Expr {
			c := b
			c.L = v
			return fc.settle(c, exact)
		}), true
	}
	if !hasSplit(b.R) {
		return nil, false
	}
	m, l := leading(b.L)
	var lets []Let
	if !pure(l) {
		m, l = 0, b.L
		l, lets = fc.hoist(l, lets)
	}
	var x Expr = restep(m, mapSplit(b.R, b.T, func(v Expr) Expr {
		c := b
		c.L, c.R = l, v
		return fc.settle(c, exact)
	}))
	for i := len(lets) - 1; i >= 0; i-- {
		x = Let{Name: lets[i].Name, Value: lets[i].Value, In: x}
	}
	return x, true
}

// hasSplit is whether x, past its steps and bindings, is a NumSplit.
func hasSplit(x Expr) bool {
	switch x := x.(type) {
	case Steps:
		return hasSplit(x.X)
	case Let:
		return hasSplit(x.In)
	case NumSplit:
		return true
	}
	return false
}

// mapSplit is x with f applied to each branch of the NumSplit it ends in, of type t.
func mapSplit(x Expr, t Type, f func(Expr) Expr) Expr {
	switch w := x.(type) {
	case Steps:
		w.X = mapSplit(w.X, t, f)
		return w
	case Let:
		w.In = mapSplit(w.In, t, f)
		return w
	case NumSplit:
		w.Int, w.Real, w.T = f(w.Int), f(w.Real), t
		return w
	}
	return f(x)
}

// settle is exact applied to b, or a run-time refusal naming why it cannot be.
func (fc *funcCompiler) settle(b Binary, exact func(Binary) (Expr, string)) Expr {
	x, why := exact(b)
	if why != "" {
		return fc.refuse(why, b.T, b.L, b.R)
	}
	return x
}

// refuse is a run-time failure of type t once operands are evaluated:
// compiled code cannot compute why.
func (fc *funcCompiler) refuse(why string, t Type, operands ...Expr) Expr {
	fc.c.collections = true
	return Refusal{Operands: operands, Parts: []string{"unsupported: " + why}, T: t}
}

// reciprocalPow is base ** e for the pure Integer base and negative Integer
// literal e: the exact Rational 1 / base ** -e, undefined for a zero base.
func (fc *funcCompiler) reciprocalPow(base Expr, e IntLit) Expr {
	n := new(big.Int).Neg(e.big())
	var den Expr = base
	if !n.IsInt64() || n.Int64() != 1 {
		den = Binary{Op: ast.OpPow, L: base, R: intLit(semantics.BigIntValue(n)), T: TypeInt}
	}
	fc.c.collections = true
	return Cond{
		C:    Binary{Op: ast.OpEq, L: base, R: IntLit{}, T: TypeBool},
		Then: Refusal{Parts: []string{fmt.Sprintf("%v: 0 ** %s is undefined (negative exponent)", semantics.ErrArithmeticDomain, e.big())}, T: TypeReal},
		Else: Binary{Op: ast.OpDiv, L: IntLit{Value: 1}, R: den, T: TypeReal},
		T:    TypeReal,
	}
}

// kindSplit is b over its one number operand split on its run-time kind: an
// Integer meets the other operand in exact, a Real in binary64 arithmetic. A
// constant other operand stays inline, so exact can fold it.
func (fc *funcCompiler) kindSplit(b Binary, exact func(Binary) (Expr, string)) Expr {
	num, other := b.L, b.R
	left := num.Type() != TypeNum
	if left {
		num, other = other, num
	}
	var m int64
	var lets []Let
	if left {
		if k, x := leading(other); pure(x) {
			m, other = k, x
		} else {
			other, lets = fc.hoist(other, lets)
		}
	}
	branch := func(ints bool) func([]Expr) Expr {
		return func(v []Expr) Expr {
			c := b
			if left {
				c.L, c.R = asReal(other), kindOperand(v[0], ints)
			} else {
				c.L, c.R = kindOperand(v[0], ints), asReal(other)
			}
			return fc.settle(c, exact)
		}
	}
	x := restep(m, fc.split([]Expr{num}, branch(true), branch(false), b.T))
	for i := len(lets) - 1; i >= 0; i-- {
		x = Let{Name: lets[i].Name, Value: lets[i].Value, In: x}
	}
	return x
}

// kindOperand is x as a Real, a number read as the Integer it holds when ints.
func kindOperand(x Expr, ints bool) Expr {
	if ints && x.Type() == TypeNum {
		return ToReal{X: AsInt{X: x}}
	}
	return asReal(x)
}

// meetsExact is whether a number operand of l and r meets one the interpreter holds exactly.
func (fc *funcCompiler) meetsExact(l, r Expr) bool {
	return (l.Type() == TypeNum && r.Type() != TypeNum && fc.exactness(r) != binary64) ||
		(r.Type() == TypeNum && l.Type() != TypeNum && fc.exactness(l) != binary64)
}

// rationalNeg is `-x` over Reals; an exact zero negates to itself.
func (fc *funcCompiler) rationalNeg(x Expr) Expr {
	if fc.exactness(x) == binary64 {
		return Unary{Op: ast.OpNeg, X: x, T: TypeReal}
	}
	if s, ok := x.(Steps); ok {
		if _, lit := bare(s.X).(RealLit); lit {
			s.X = fc.rationalNeg(s.X)
			return s
		}
	}
	if lit, ok := x.(RealLit); ok && lit.Rat != nil {
		return RealLit{Value: 0 - lit.Value, Rat: new(big.Rat).Neg(lit.Rat)}
	}
	return Unary{Op: ast.OpNeg, X: x, T: TypeReal, Exact: true}
}

// exactOperands refuses the collection operation what over Real elements the
// interpreter holds exactly.
func (fc *funcCompiler) exactOperands(what string, operands ...Expr) error {
	for _, x := range operands {
		if x.Type().Elem() == TypeReal && fc.exactness(x) != binary64 {
			return fc.unsupported(fmt.Sprintf("%s over exact Rationals (compiled code holds Rationals as binary64)", what))
		}
	}
	return nil
}
