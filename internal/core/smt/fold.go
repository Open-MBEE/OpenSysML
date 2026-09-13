package smt

import (
	"math"

	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
)

// The encoder's constructors fold literal operands away: much of the relation
// is fixed by the graph, and the solver should not be handed the dead branches.

// literal reports whether t is a Bool, Int or datatype literal.
func literal(t *solve.Term) bool {
	switch t.Op {
	case solve.OpBool, solve.OpInt, solve.OpValue:
		return true
	}
	return false
}

// isBool reports whether t is the boolean literal b.
func isBool(t *solve.Term, b bool) bool { return t.Op == solve.OpBool && t.Bool == b }

// same reports whether a and b are the same literal or read the same variable.
func same(a, b *solve.Term) bool {
	if a == b {
		return true
	}
	if a.Op != b.Op {
		return false
	}
	switch a.Op {
	case solve.OpBool:
		return a.Bool == b.Bool
	case solve.OpInt:
		return a.Int == b.Int
	case solve.OpValue:
		return a.Sort.Equal(b.Sort) && a.Str == b.Str
	case solve.OpVar:
		return a.Var == b.Var
	}
	return false
}

// and is the conjunction, dropping `true` and yielding `false` on `false`.
func and(args ...*solve.Term) *solve.Term {
	kept := make([]*solve.Term, 0, len(args))
	for _, a := range args {
		if isBool(a, false) {
			return solve.BoolTerm(false)
		}
		if !isBool(a, true) {
			kept = append(kept, a)
		}
	}
	return solve.And(kept...)
}

// or is the disjunction, dropping `false` and yielding `true` on `true`.
func or(args ...*solve.Term) *solve.Term {
	kept := make([]*solve.Term, 0, len(args))
	for _, a := range args {
		if isBool(a, true) {
			return solve.BoolTerm(true)
		}
		if !isBool(a, false) {
			kept = append(kept, a)
		}
	}
	return solve.Or(kept...)
}

// not is the negation, folded over a literal.
func not(a *solve.Term) *solve.Term {
	if a.Op == solve.OpBool {
		return solve.BoolTerm(!a.Bool)
	}
	return solve.Not(a)
}

// implies is implication, folded when either side is decided.
func implies(a, b *solve.Term) *solve.Term {
	switch {
	case isBool(a, false), isBool(b, true):
		return solve.BoolTerm(true)
	case isBool(a, true):
		return b
	case isBool(b, false):
		return not(a)
	}
	return solve.Binary(solve.OpImplies, solve.Bool, a, b)
}

// ite is the conditional, folded when the condition or the branches decide it.
func ite(cond, then, otherwise *solve.Term) *solve.Term {
	switch {
	case isBool(cond, true):
		return then
	case isBool(cond, false):
		return otherwise
	case same(then, otherwise):
		return then
	}
	return solve.Ite(cond, then, otherwise)
}

// eq is equality of two terms of one sort, decided between literals or the
// same variable, and reduced against a boolean literal.
func eq(a, b *solve.Term) *solve.Term {
	switch {
	case same(a, b):
		return solve.BoolTerm(true)
	case literal(a) && literal(b):
		return solve.BoolTerm(false)
	case a.Op == solve.OpBool:
		return eq(b, a)
	case isBool(b, true):
		return a
	case isBool(b, false):
		return not(a)
	}
	return solve.Binary(solve.OpEq, solve.Bool, a, b)
}

// add is integer addition, folded over literals that stay within int64 and a
// zero operand.
func add(a, b *solve.Term) *solve.Term {
	switch {
	case a.Op == solve.OpInt && b.Op == solve.OpInt && !overflows(a.Int, b.Int, a.Int+b.Int):
		return solve.IntTerm(a.Int + b.Int)
	case a.Op == solve.OpInt && a.Int == 0:
		return b
	case b.Op == solve.OpInt && b.Int == 0:
		return a
	}
	return solve.Binary(solve.OpAdd, solve.Int, a, b)
}

// sub is integer subtraction, folded over literals that stay within int64 and
// a zero subtrahend.
func sub(a, b *solve.Term) *solve.Term {
	switch {
	case a.Op == solve.OpInt && b.Op == solve.OpInt && !overflows(a.Int, -b.Int, a.Int-b.Int):
		return solve.IntTerm(a.Int - b.Int)
	case b.Op == solve.OpInt && b.Int == 0:
		return a
	}
	return solve.Binary(solve.OpSub, solve.Int, a, b)
}

// overflows reports whether sum, computed as a+b in int64, wrapped; b is
// negated for a difference, so math.MinInt64 is never folded away.
func overflows(a, b, sum int64) bool {
	return b == math.MinInt64 || (b > 0 && sum < a) || (b < 0 && sum > a)
}

// gt is integer `>`, decided between literals.
func gt(a, b *solve.Term) *solve.Term {
	if a.Op == solve.OpInt && b.Op == solve.OpInt {
		return solve.BoolTerm(a.Int > b.Int)
	}
	return solve.Binary(solve.OpGt, solve.Bool, a, b)
}

// ge is integer `>=`, decided between literals.
func ge(a, b *solve.Term) *solve.Term {
	if a.Op == solve.OpInt && b.Op == solve.OpInt {
		return solve.BoolTerm(a.Int >= b.Int)
	}
	return solve.Binary(solve.OpGe, solve.Bool, a, b)
}
