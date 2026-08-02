package ast

import "github.com/Open-MBEE/Systemica/internal/core/source"

// OperatorKind enumerates every operator in the KerMLExpressions ladder,
// used by OperatorExpr regardless of arity.
type OperatorKind int

const (
	OpInvalid        OperatorKind = iota
	OpConditional                 // if C ? A else B
	OpNullCoalesce                // ??
	OpImplies                     // implies
	OpOr                          // |
	OpConditionalOr               // or
	OpXor                         // xor
	OpAnd                         // &
	OpConditionalAnd              // and
	OpEq                          // ==
	OpNeq                         // !=
	OpEqEqEq                      // ===
	OpNeqEqEq                     // !==
	OpHasType                     // hastype
	OpIsType                      // istype
	OpAt                          // @
	OpMetaAt                      // @@
	OpAs                          // as
	OpMeta                        // meta
	OpLt                          // <
	OpGt                          // >
	OpLe                          // <=
	OpGe                          // >=
	OpRange                       // ..
	OpAdd                         // +
	OpSub                         // -
	OpMul                         // *
	OpDiv                         // /
	OpMod                         // %
	OpPow                         // ** or ^
	OpNeg                         // unary -
	OpPos                         // unary +
	OpBitNot                      // unary ~
	OpNot                         // unary not
	OpAll                         // extent: all
	OpIndex                       // [ ... ]
)

var operatorNames = map[OperatorKind]string{
	OpConditional: "if", OpNullCoalesce: "??", OpImplies: "implies",
	OpOr: "|", OpConditionalOr: "or", OpXor: "xor", OpAnd: "&",
	OpConditionalAnd: "and", OpEq: "==", OpNeq: "!=", OpEqEqEq: "===",
	OpNeqEqEq: "!==", OpHasType: "hastype", OpIsType: "istype", OpAt: "@",
	OpMetaAt: "@@", OpAs: "as", OpMeta: "meta", OpLt: "<", OpGt: ">",
	OpLe: "<=", OpGe: ">=", OpRange: "..", OpAdd: "+", OpSub: "-",
	OpMul: "*", OpDiv: "/", OpMod: "%", OpPow: "**", OpNeg: "-",
	OpPos: "+", OpBitNot: "~", OpNot: "not", OpAll: "all", OpIndex: "[]",
}

func (k OperatorKind) String() string {
	if s, ok := operatorNames[k]; ok {
		return s
	}
	return "OperatorKind(?)"
}

// LiteralBool is `true`/`false`.
type LiteralBool struct {
	NodeBase
	Value bool
}

// LiteralString is a double-quoted string literal (raw token text, quotes included).
type LiteralString struct {
	NodeBase
	Value string
}

// LiteralInteger is a DECIMAL_VALUE literal (raw text).
type LiteralInteger struct {
	NodeBase
	Value string
}

// LiteralReal is a RealValue literal (raw text).
type LiteralReal struct {
	NodeBase
	Value string
}

// LiteralInfinity is `*` in an expression position.
type LiteralInfinity struct{ NodeBase }

// NullExpr is `null` or `( )`.
type NullExpr struct{ NodeBase }

// FeatureReference is a bare QualifiedName used as an expression.
type FeatureReference struct {
	NodeBase
	Name *QualifiedName
}

// OperatorExpr is any operator application. Operands has 3 elements for
// OpConditional (cond, then, else), 1 for unary/extent, 2 otherwise.
// For OpAt/OpMetaAt/OpAs/OpMeta the RHS type reference is stored in TypeRef.
type OperatorExpr struct {
	NodeBase
	Operator OperatorKind
	Operands []Node
	TypeRef  *QualifiedName // classification/cast RHS, else nil
}

// FeatureChainExpr is `operand . member` (feature chain access).
type FeatureChainExpr struct {
	NodeBase
	Operand Node
	Member  *QualifiedName
}

// IndexExpr is `operand # ( seq )`.
type IndexExpr struct {
	NodeBase
	Operand Node
	Index   Node
}

// InvocationExpr is `Type ( args )` or `operand -> Type ( args | body | funcref )`.
type InvocationExpr struct {
	NodeBase
	Operand   Node           // receiver for `->` form, else nil
	Type      *QualifiedName // instantiated type
	Args      []Node         // positional args (Argument) or ...
	NamedArgs []NamedArg     // named args, mutually exclusive with Args
}

// NamedArg is `name = value` in an argument list.
type NamedArg struct {
	Name  *QualifiedName
	Value Node
}

// CollectExpr is `operand . body`.
type CollectExpr struct {
	NodeBase
	Operand Node
	Body    Node
}

// SelectExpr is `operand .? body`.
type SelectExpr struct {
	NodeBase
	Operand Node
	Body    Node
}

// ConstructorExpr is `new Type ( args )`.
type ConstructorExpr struct {
	NodeBase
	Type *QualifiedName
	Args []Node
}

// BodyExpr is `{ (in param ;)* resultExpr }`.
type BodyExpr struct {
	NodeBase
	Params []BodyParam
	Result Node
}

// BodyParam is `in name` inside a body expression.
type BodyParam struct {
	Name            string
	Type            *QualifiedName // optional type annotation (e.g., in x : Type)
	RedefinesTarget *QualifiedName // optional redefines target (e.g., in x :> Parent)
	Multiplicity    *Multiplicity  // optional multiplicity after type (e.g., in x : Type [1])
	Value           Node           // optional default value (e.g., in x = expr)
	IsReference     bool           // true if 'ref' modifier present
	Members         []Node         // optional body members (e.g., in x { doc ... })
	Span            source.Span
}

// SequenceExpr is a comma-separated list of expressions (flattened).
type SequenceExpr struct {
	NodeBase
	Elements []Node
}

// MetadataAccessExpr is `ref . metadata`.
type MetadataAccessExpr struct {
	NodeBase
	Ref *QualifiedName
}
