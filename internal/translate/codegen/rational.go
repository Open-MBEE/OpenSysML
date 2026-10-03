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
		return join(fc.classify(x.Then, reached), fc.classify(x.Else, reached))
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
	switch x := x.(type) {
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
	l, r := fc.exactness(b.L), fc.exactness(b.R)
	if l == binary64 || r == binary64 {
		return b, nil
	}
	if b.Op == ast.OpPow {
		if !isWidenedInt(b.R) {
			return b, nil
		}
		if a, okA := constRat(b.L); okA {
			if n, okN := b.R.(ToReal).X.(IntLit); okN && n.Big == nil {
				if v, err := semantics.RatPow(semantics.RatValue(a), semantics.IntValue(n.Value), semantics.DefaultMaxIntegerBits); err == nil {
					if lit, ok := exactLit(v.Rat()); ok {
						return lit, nil
					}
				}
			}
		}
		return nil, fc.unsupported("'**' of an exact Rational by an Integer exponent (compiled code holds Rationals as binary64)")
	}
	if a, okA := constRat(b.L); okA {
		if c, okC := constRat(b.R); okC && b.Op != ast.OpMod {
			if v, err := semantics.RatArith(b.Op, semantics.RatValue(a), semantics.RatValue(c), semantics.DefaultMaxIntegerBits); err == nil {
				if lit, ok := exactLit(v.Rat()); ok {
					return lit, nil
				}
			}
		}
	}
	if b.Op == ast.OpDiv && b.L.Type() == TypeInt {
		return b, nil
	}
	if heldExactly(l) && heldExactly(r) {
		if b.Op == ast.OpMul && (scalesExactly(b.L) || scalesExactly(b.R)) {
			// Scaling by 2^k, k >= 0, commutes with rounding, so neither operand needs a guard.
		} else {
			b.L, b.R = exactWiden(b.L), exactWiden(b.R)
		}
		b.Exact = true
		b.Whole = l == exactWhole && r == exactWhole && (b.Op == ast.OpAdd || b.Op == ast.OpSub || b.Op == ast.OpMul)
		return b, nil
	}
	return nil, fc.unsupported(fmt.Sprintf("exact Rational arithmetic '%s' over a value binary64 does not hold exactly (compiled code holds Rationals as binary64)", b.Op))
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
	if b.L.Type() != TypeReal || b.R.Type() != TypeReal {
		return b, nil
	}
	l, r := fc.exactness(b.L), fc.exactness(b.R)
	if l != exactRational && r != exactRational {
		if l != binary64 {
			b.L = exactWiden(b.L)
		}
		if r != binary64 {
			b.R = exactWiden(b.R)
		}
		return b, nil
	}
	if q, ok := intQuotient(b.L); ok && r == exactWhole {
		b.L, b.R = q, exactWiden(b.R)
		return b, nil
	}
	if q, ok := intQuotient(b.R); ok && l == exactWhole {
		b.L, b.R = exactWiden(b.L), q
		return b, nil
	}
	if a, okA := constRat(b.L); okA {
		if c, okC := constRat(b.R); okC {
			return BoolLit{Value: compares(b.Op, a.Cmp(c))}, nil
		}
	}
	if c, ok := constRat(b.R); ok && l == binary64 {
		return againstNearest(b.Op, b.L, c), nil
	}
	if c, ok := constRat(b.L); ok && r == binary64 {
		return againstNearest(flipped(b.Op), b.R, c), nil
	}
	return nil, fc.unsupported(fmt.Sprintf("'%s' of an exact Rational binary64 does not hold exactly (compiled code holds Rationals as binary64)", b.Op))
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
// r, which is finite since a literal or fold past the range is refused.
func againstNearest(op ast.OperatorKind, x Expr, r *big.Rat) Expr {
	d, _ := r.Float64()
	return Binary{Op: op, L: x, R: RealLit{Value: d}, T: TypeBool}
}

// rationalNeg is `-x` over Reals; an exact zero negates to itself.
func (fc *funcCompiler) rationalNeg(x Expr) Expr {
	if fc.exactness(x) == binary64 {
		return Unary{Op: ast.OpNeg, X: x, T: TypeReal}
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
