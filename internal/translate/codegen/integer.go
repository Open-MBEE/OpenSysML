package codegen

import (
	"math/big"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// intLit is the literal of the Integer v.
func intLit(v semantics.Value) IntLit {
	if n, ok := v.Int64(); ok {
		return IntLit{Value: n}
	}
	return IntLit{Big: v.BigInt()}
}

func (l IntLit) big() *big.Int {
	if l.Big != nil {
		return l.Big
	}
	return big.NewInt(l.Value)
}

func (l IntLit) sign() int {
	if l.Big != nil {
		return l.Big.Sign()
	}
	switch {
	case l.Value < 0:
		return -1
	case l.Value > 0:
		return 1
	}
	return 0
}

func isComparison(op ast.OperatorKind) bool {
	switch op {
	case ast.OpLt, ast.OpLe, ast.OpGt, ast.OpGe, ast.OpEq, ast.OpNeq:
		return true
	}
	return false
}

// widenedInt is the scalar Integer x widens to a Real, if it does.
func widenedInt(x Expr) (Expr, bool) {
	if w, ok := x.(ToReal); ok && w.X.Type() == TypeInt {
		return w.X, true
	}
	return nil, false
}

// exactQuotient is x as an Integer quotient compared exactly, if it is one.
func exactQuotient(x Expr) (Binary, bool) {
	q, ok := x.(Binary)
	return q, ok && q.Exact && q.Op == ast.OpDiv && q.L.Type() == TypeInt
}

// wholeOperand is the whole number x as an Integer where it widens one.
func wholeOperand(x Expr) Expr {
	if i, ok := widenedInt(x); ok {
		return i
	}
	return x
}

func isWidenedInt(x Expr) bool {
	_, ok := widenedInt(x)
	return ok
}
