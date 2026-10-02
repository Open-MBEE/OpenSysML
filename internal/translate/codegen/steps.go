package codegen

import (
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// stepped is x, the compiled node n, charged the step the interpreter spends
// evaluating n. An operator the interpreter folds to a constant spends that one
// step and none for its operands.
func (fc *funcCompiler) stepped(n ast.Node, x Expr) Expr {
	if op, ok := n.(*ast.OperatorExpr); ok {
		if _, folds := fc.c.model.EvalWithin(op, runtime.DefaultMaxIntegerBits); folds {
			return Steps{N: 1, X: unstepped(x)}
		}
	}
	return charged(1, x)
}

// charged is x spending n steps first, merged with the charges x makes before
// anything it evaluates can fail, so a run spends them in one count.
func charged(n int64, x Expr) Expr {
	m, rest := leading(x)
	return Steps{N: n + m, X: rest}
}

// leading splits off the steps x spends before its first evaluation that can
// fail or branch, returning them and x without them.
func leading(x Expr) (int64, Expr) {
	switch x := x.(type) {
	case Steps:
		m, rest := leading(x.X)
		return x.N + m, rest
	case ToReal:
		m, rest := leading(x.X)
		x.X = rest
		return m, x
	case ToNum:
		m, rest := leading(x.X)
		x.X = rest
		return m, x
	case ToMany:
		m, rest := leading(x.X)
		x.X = rest
		return m, x
	case Unary:
		m, rest := leading(x.X)
		x.X = rest
		return m, x
	case Cond:
		m, rest := leading(x.C)
		x.C = rest
		return m, x
	case Binary:
		m, l := leading(x.L)
		x.L = l
		if infallible(l) && !lazy(x.Op) {
			k, r := leading(x.R)
			x.R = r
			m += k
		}
		return m, x
	case Call:
		var m int64
		x.Args, m = leadingArgs(x.Args)
		return m, x
	case LibCall:
		var m int64
		x.Args, m = leadingArgs(x.Args)
		return m, x
	}
	return 0, x
}

// leadingArgs lifts the leading steps of arguments evaluated before any can fail.
func leadingArgs(args []Arg) ([]Arg, int64) {
	out := append([]Arg(nil), args...)
	var total int64
	for i := range out {
		m, v := leading(out[i].Value)
		out[i].Value = v
		total += m
		if !infallible(v) {
			break
		}
	}
	return out, total
}

func lazy(op ast.OperatorKind) bool {
	switch op {
	case ast.OpAnd, ast.OpConditionalAnd, ast.OpOr, ast.OpConditionalOr, ast.OpImplies:
		return true
	}
	return false
}

// infallible is an expression that spends no step and cannot fail.
func infallible(x Expr) bool {
	switch x := x.(type) {
	case IntLit, RealLit, BoolLit, Var, NullLit:
		return true
	case ToReal:
		return infallible(x.X)
	case ToNum:
		return infallible(x.X)
	}
	return false
}

// unstepped is a folded operator's compiled tree without its operands' steps;
// such a tree holds only literals and scalar operators.
func unstepped(x Expr) Expr {
	switch x := x.(type) {
	case Steps:
		return unstepped(x.X)
	case ToReal:
		x.X = unstepped(x.X)
		return x
	case ToNum:
		x.X = unstepped(x.X)
		return x
	case Unary:
		x.X = unstepped(x.X)
		return x
	case Binary:
		x.L, x.R = unstepped(x.L), unstepped(x.R)
		return x
	case Cond:
		x.C, x.Then, x.Else = unstepped(x.C), unstepped(x.Then), unstepped(x.Else)
		return x
	}
	return x
}

// bare is x without the steps it spends first.
func bare(x Expr) Expr {
	if s, ok := x.(Steps); ok {
		return bare(s.X)
	}
	return x
}
