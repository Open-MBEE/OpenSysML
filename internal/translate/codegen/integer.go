package codegen

import (
	"fmt"

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

// cIntegerRefusal refuses a program for the C target when it computes an
// Integer that may leave int64. The interpreter's Integers are unbounded and
// C has no arbitrary-precision integer of its own, so a C program holds
// int64 only where no result can leave it: Integer literals, comparisons,
// sizes, indexes, ranges, Integer `/` (a Real) and `%`, min and max, and
// arithmetic over literals whose result fits.
func cIntegerRefusal(p *Program) error {
	for _, fn := range p.Funcs {
		if what := unboundedIntStmts(fn.Body); what != "" {
			return &UnsupportedError{Calc: fn.Name, What: what + " for the C target: the interpreter's Integers are unbounded and a C program holds int64 (the Go target computes them exactly)"}
		}
	}
	return nil
}

func unboundedIntStmts(stmts []Stmt) string {
	for _, s := range stmts {
		if what := unboundedIntStmt(s); what != "" {
			return what
		}
	}
	return ""
}

func unboundedIntStmt(s Stmt) string {
	switch s := s.(type) {
	case Declare:
		return unboundedIntExprs(s.Init)
	case Assign:
		return unboundedIntExprs(s.Value)
	case If:
		if what := unboundedIntExprs(s.Cond); what != "" {
			return what
		}
		if what := unboundedIntStmts(s.Then); what != "" {
			return what
		}
		return unboundedIntStmts(s.Else)
	case While:
		if what := unboundedIntExprs(s.Cond, s.Until); what != "" {
			return what
		}
		return unboundedIntStmts(s.Body)
	case ForEach:
		if what := unboundedIntExprs(s.Seq); what != "" {
			return what
		}
		return unboundedIntStmts(s.Body)
	case Sample:
		return unboundedIntExprs(s.Seq, s.Body.Body)
	case Return:
		return unboundedIntExprs(s.Value)
	}
	return fmt.Sprintf("statement %T", s)
}

func unboundedIntExprs(xs ...Expr) string {
	for _, x := range xs {
		if x == nil {
			continue
		}
		if what := unboundedIntExpr(x); what != "" {
			return what
		}
	}
	return ""
}

// unboundedIntExpr names the first construct of x, in evaluation order, whose
// Integer result may leave int64, or is empty.
func unboundedIntExpr(x Expr) string {
	switch x := x.(type) {
	case IntLit:
		if x.Big != nil {
			return fmt.Sprintf("the Integer literal %s, beyond int64,", x.Big)
		}
	case RealLit, BoolLit, Var, NullLit:
	case Binary:
		if what := unboundedIntExprs(x.L, x.R); what != "" {
			return what
		}
		return unboundedIntBinary(x)
	case Unary:
		if what := unboundedIntExprs(x.X); what != "" || x.Op != ast.OpNeg || x.T != TypeInt {
			return what
		}
		return "Integer negation"
	case Cond:
		return unboundedIntExprs(x.C, x.Then, x.Else)
	case Call:
		return unboundedIntExprs(argValues(x.Args)...)
	case LibCall:
		if what := unboundedIntExprs(argValues(x.Args)...); what != "" {
			return what
		}
		switch x.Op {
		case LibAbsInt, LibFloor, LibRound:
			return fmt.Sprintf("the Integer result of %s", x.Op)
		}
	case ToReal:
		return unboundedIntExprs(x.X)
	case SeqLit:
		return unboundedIntExprs(x.Elems...)
	case ToMany:
		return unboundedIntExprs(x.X)
	case ToOne:
		return unboundedIntExprs(x.X, x.Other)
	case Checked:
		return unboundedIntExprs(x.X)
	case Let:
		return unboundedIntExprs(x.Value, x.In)
	case Coalesce:
		return unboundedIntExprs(x.L, x.R)
	case SeqEq:
		return unboundedIntExprs(x.L, x.R)
	case Index:
		return unboundedIntExprs(x.Seq, x.I)
	case RangeExpr:
		return unboundedIntExprs(x.Lo, x.Hi)
	case SeqCall:
		if what := unboundedIntExprs(x.Args...); what != "" {
			return what
		}
		if (x.Op == SeqSum || x.Op == SeqProduct) && x.T == TypeInt {
			return fmt.Sprintf("the Integer %s", x.Op)
		}
	case Fold:
		return unboundedIntExprs(x.Seq, x.Body.Body)
	case Framed:
		return unboundedIntExprs(x.X)
	case Sampled:
		return unboundedIntExprs(x.S.Seq, x.S.Body.Body, x.In)
	default:
		return fmt.Sprintf("expression %T", x)
	}
	return ""
}

// unboundedIntBinary names an Integer `+`, `-`, `*` or `**` not provably
// within int64: one over literals whose result fits, or a power by 0 or 1, is.
func unboundedIntBinary(x Binary) string {
	switch x.Op {
	case ast.OpAdd, ast.OpSub, ast.OpMul:
		if x.L.Type() != TypeInt {
			return ""
		}
		l, lok := x.L.(IntLit)
		r, rok := x.R.(IntLit)
		if lok && rok && l.Big == nil && r.Big == nil {
			res, err := semantics.IntArith(x.Op, semantics.IntValue(l.Value), semantics.IntValue(r.Value), semantics.DefaultMaxIntegerBits)
			if _, fits := res.Int64(); err == nil && fits {
				return ""
			}
		}
	case ast.OpPow:
		if x.T != TypeInt {
			return ""
		}
		if n, ok := x.R.(IntLit); ok && n.Big == nil && (n.Value == 0 || n.Value == 1) {
			return ""
		}
	default:
		return ""
	}
	return fmt.Sprintf("Integer `%s`", x.Op)
}
