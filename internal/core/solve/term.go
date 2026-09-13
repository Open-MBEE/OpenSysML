package solve

import "math/big"

// Op is the operator a term applies. The set is closed: every translatable
// construct maps onto one of these, and anything else refuses with
// ErrNotTranslatable.
type Op int

const (
	// OpBool is a boolean literal, held in Bool.
	OpBool Op = iota
	// OpInt is an integer literal, held in Int.
	OpInt
	// OpReal is a real literal, held as an exact rational in Real.
	OpReal
	// OpString is a string literal, held in Str.
	OpString
	// OpValue is a value of a datatype sort — an enumeration literal or a
	// variant — named by Str.
	OpValue
	// OpVar is a variable, held in Var.
	OpVar

	// OpNot is boolean negation, one argument.
	OpNot
	// OpAnd is conjunction, two or more arguments.
	OpAnd
	// OpOr is disjunction, two or more arguments.
	OpOr
	// OpXor is exclusive disjunction, two arguments.
	OpXor
	// OpImplies is implication, two arguments.
	OpImplies

	// OpEq is equality of two terms of the same sort.
	OpEq
	// OpNe is inequality of two terms of the same sort.
	OpNe
	// OpLt is `<` between two numbers of the same sort.
	OpLt
	// OpLe is `<=` between two numbers of the same sort.
	OpLe
	// OpGt is `>` between two numbers of the same sort.
	OpGt
	// OpGe is `>=` between two numbers of the same sort.
	OpGe

	// OpAdd is addition of two numbers of the same sort.
	OpAdd
	// OpSub is subtraction of two numbers of the same sort.
	OpSub
	// OpMul is multiplication of two numbers of the same sort.
	OpMul
	// OpDiv is division of two reals.
	OpDiv
	// OpIntDiv is SMT-LIB's Euclidean integer division; TruncRem builds the
	// evaluator's truncating remainder from it.
	OpIntDiv
	// OpNeg is arithmetic negation, one argument.
	OpNeg

	// OpIte is `if c then a else b`: a boolean condition and two branches of
	// the same sort.
	OpIte
	// OpToReal widens an integer term to a real one.
	OpToReal
	// OpInt64 is whether an integer term lies within int64, one argument: where
	// the evaluator computes it without reporting overflow.
	OpInt64
)

// smtOps names the SMT-LIB operator each compound term is written with.
var smtOps = map[Op]string{
	OpNot: "not", OpAnd: "and", OpOr: "or", OpXor: "xor", OpImplies: "=>",
	OpEq: "=", OpNe: "distinct", OpLt: "<", OpLe: "<=", OpGt: ">", OpGe: ">=",
	OpAdd: "+", OpSub: "-", OpMul: "*", OpDiv: "/", OpIntDiv: "div", OpNeg: "-",
	OpIte: "ite", OpToReal: "to_real",
}

// Term is one node of the query's term language: an operator, the sort of the
// value it yields, and its arguments. A leaf carries its value instead of
// arguments. Terms are built by the translator and are never mutated afterwards.
type Term struct {
	// Op is the operator applied.
	Op Op

	// Sort is the sort of the value the term yields.
	Sort Sort

	// Args are the operands of a compound term, nil for a leaf.
	Args []*Term

	// Var is the variable an OpVar term reads.
	Var *Var

	// Bool holds an OpBool literal.
	Bool bool

	// Int holds an OpInt literal.
	Int int64

	// Real holds an OpReal literal as an exact rational.
	Real *big.Rat

	// Str holds an OpString literal or the name of an OpValue datatype value.
	Str string

	// IntRatio marks an OpDiv over widened integers: a whole-number quotient,
	// which the evaluator computes as the exact ratio rounded once to float64.
	IntRatio bool
}

// Literal reports whether the term is a literal value, which is what makes a
// product or a quotient linear.
func (t *Term) Literal() bool {
	switch t.Op {
	case OpBool, OpInt, OpReal, OpString, OpValue:
		return true
	}
	return false
}

// BoolTerm returns a boolean literal.
func BoolTerm(b bool) *Term { return &Term{Op: OpBool, Sort: Bool, Bool: b} }

// IntTerm returns an integer literal.
func IntTerm(i int64) *Term { return &Term{Op: OpInt, Sort: Int, Int: i} }

// RealTerm returns a real literal, held as the exact rational given.
func RealTerm(r *big.Rat) *Term { return &Term{Op: OpReal, Sort: Real, Real: new(big.Rat).Set(r)} }

// StringTerm returns a string literal.
func StringTerm(s string) *Term { return &Term{Op: OpString, Sort: String, Str: s} }

// ValueTerm returns a value of a datatype sort, named by one of its values.
func ValueTerm(sort Sort, value string) *Term {
	return &Term{Op: OpValue, Sort: sort, Str: value}
}

// RatioDiv returns the exact real quotient of two integer terms, marked as a
// whole-number ratio for the evaluator-arithmetic replay.
func RatioDiv(a, b *Term) *Term {
	t := Binary(OpDiv, Real, ToReal(a), ToReal(b))
	t.IntRatio = true
	return t
}

// VarTerm returns a term reading a variable.
func VarTerm(v *Var) *Term { return &Term{Op: OpVar, Sort: v.Sort, Var: v} }

// Not returns the negation of a boolean term, folding a double negation.
func Not(arg *Term) *Term {
	if arg.Op == OpNot {
		return arg.Args[0]
	}
	return &Term{Op: OpNot, Sort: Bool, Args: []*Term{arg}}
}

// And returns the conjunction of boolean terms: `true` for none, the term itself
// for one.
func And(args ...*Term) *Term { return junction(OpAnd, true, args) }

// Or returns the disjunction of boolean terms: `false` for none, the term itself
// for one.
func Or(args ...*Term) *Term { return junction(OpOr, false, args) }

// junction builds a conjunction or a disjunction, with the identity of the
// operator standing for an empty one.
func junction(op Op, identity bool, args []*Term) *Term {
	switch len(args) {
	case 0:
		return BoolTerm(identity)
	case 1:
		return args[0]
	}
	return &Term{Op: op, Sort: Bool, Args: args}
}

// Binary returns a compound term over two arguments, of the sort given.
func Binary(op Op, sort Sort, left, right *Term) *Term {
	return &Term{Op: op, Sort: sort, Args: []*Term{left, right}}
}

// Unary returns a compound term over one argument, of the sort given.
func Unary(op Op, sort Sort, arg *Term) *Term {
	return &Term{Op: op, Sort: sort, Args: []*Term{arg}}
}

// Ite returns a conditional term: cond selects between two branches, which must
// share the sort the term yields.
func Ite(cond, then, otherwise *Term) *Term {
	return &Term{Op: OpIte, Sort: then.Sort, Args: []*Term{cond, then, otherwise}}
}

// ToReal widens an integer term to a real one, leaving a real term as it is.
func ToReal(arg *Term) *Term {
	if arg.Sort.Kind == SortReal {
		return arg
	}
	if arg.Op == OpInt {
		return RealTerm(new(big.Rat).SetInt64(arg.Int))
	}
	return &Term{Op: OpToReal, Sort: Real, Args: []*Term{arg}}
}

// Int64 returns whether an integer term lies within int64.
func Int64(arg *Term) *Term { return &Term{Op: OpInt64, Sort: Bool, Args: []*Term{arg}} }

// TruncDiv returns integer division truncating toward zero:
// `ite(a >= 0, div(a, b), -div(-a, b))`. TruncRem builds the remainder from it.
func TruncDiv(a, b *Term) *Term {
	positive := Binary(OpIntDiv, Int, a, b)
	negative := Unary(OpNeg, Int, Binary(OpIntDiv, Int, Unary(OpNeg, Int, a), b))
	return Ite(Binary(OpGe, Bool, a, IntTerm(0)), positive, negative)
}

// TruncRem returns the remainder that truncating division leaves,
// `a - b*TruncDiv(a, b)`, which takes the sign of the dividend as `%` does.
func TruncRem(a, b *Term) *Term {
	product := Binary(OpMul, Int, b, TruncDiv(a, b))
	return Binary(OpSub, Int, a, product)
}

// walk visits the term and every term below it, parents first.
func (t *Term) walk(visit func(*Term)) {
	visit(t)
	for _, arg := range t.Args {
		arg.walk(visit)
	}
}
