package program

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// ExprKind is what an expression is, which the evaluators switch on.
type ExprKind uint8

const (
	// ExprNone is the zero kind: syntax no expression kind covers.
	ExprNone ExprKind = iota
	// ExprInt is an Integer literal, `42`.
	ExprInt
	// ExprReal is a decimal literal, `0.1`, read as the exact Rational it denotes.
	ExprReal
	// ExprBool is `true` or `false`.
	ExprBool
	// ExprString is a string literal.
	ExprString
	// ExprInfinity is `*` in expression position.
	ExprInfinity
	// ExprNull is `null` or `()`.
	ExprNull
	// ExprRef is a name read as what it names, `rate` or `A::B::x`.
	ExprRef
	// ExprChain is one read from an operand through every member in turn, `a.b.c`.
	ExprChain
	// ExprOp is an operator applied to its operands.
	ExprOp
	// ExprSequence is `(a, b, c)`.
	ExprSequence
	// ExprIndex is `seq#(i)` or the quantity form `magnitude [unit]`.
	ExprIndex
	// ExprCollect is `seq.{body}`.
	ExprCollect
	// ExprSelect is `seq.?{body}`.
	ExprSelect
	// ExprInvoke is `F(args)` or `operand->F(args)`.
	ExprInvoke
	// ExprConstruct is `new T(args)`.
	ExprConstruct
	// ExprBody is a body expression, `{ in x; x + 1 }`.
	ExprBody
	// ExprMetadata is `ref.metadata`.
	ExprMetadata
)

// Expr is one lowered expression: its kind and, by kind, its operands, the names
// it reads as lowering resolved them, or the constant it spells.
type Expr struct {
	Kind ExprKind
	// Op is the operator an ExprOp applies.
	Op Operator
	// Bracket marks an ExprIndex written `operand [unit]`, the quantity form.
	Bracket bool
	// Global marks an ExprRef whose name began with `$::`.
	Global bool
	// Origin is where the expression was written.
	Origin Origin
	// Args are the operands of an operator, the elements of a sequence, an index's
	// operand then index, and a chain's first operand.
	Args []ExprID
	// Ref is what an ExprRef names, and the type an ExprOp classifies or casts by.
	Ref SymbolRef
	// Path is the segments of an ExprRef written as a qualified name, each with
	// the element the name reaches there; nil for a simple name.
	Path []SymbolRef
	// Chain is the members an ExprChain reads after its operand, in order.
	Chain []SymbolRef
	// Const is the constant a literal spells; its Kind is invalid for a numeric
	// literal whose spelling is none or exceeds the default size budget.
	Const semantics.Value
	// Text is a numeric literal as written, and an ExprString's decoded text.
	Text string
}

// SymbolRef is a name as lowering resolved it: the element it denotes in the
// scope the expression was lowered in, nil where only a run can tell.
type SymbolRef struct {
	Sym *symbols.Symbol
	// Name is the name as written, which a run resolves by where Sym is nil.
	Name string
	Span source.Span
}
