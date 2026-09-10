// Package codegen compiles the value subset of SysML v2 calcs to native code;
// see docs/project/native-compilation.md for the subset and its semantics.
package codegen

import "github.com/Open-MBEE/OpenSysML/internal/core/ast"

// Type is a type of the compiled subset: a scalar, or a collection of scalars.
// A collection value is the interpreter's dynamic view of a multi-valued
// feature: null, one bare scalar, or a sequence of any length (its shape).
type Type int

const (
	TypeInvalid Type = iota
	TypeInt          // Integer and its subtypes, int64 with reported overflow
	TypeReal         // Real and Rational, IEEE 754 binary64
	TypeBool
	TypeNull    // `null` before context fixes its collection type
	TypeSeqInt  // collection of Integers
	TypeSeqReal // collection of Reals
	TypeSeqBool // collection of Booleans
)

func (t Type) String() string {
	switch t {
	case TypeInt:
		return "Integer"
	case TypeReal:
		return "Real"
	case TypeBool:
		return "Boolean"
	case TypeNull:
		return "null"
	case TypeSeqInt, TypeSeqReal, TypeSeqBool:
		return t.Elem().String() + "[0..*]"
	}
	return "invalid"
}

// Scalar reports whether t is exactly one Integer, Real or Boolean.
func (t Type) Scalar() bool { return t >= TypeInt && t <= TypeBool }

// Many reports whether t is a collection type.
func (t Type) Many() bool { return t >= TypeSeqInt }

// Elem is the scalar type of t's values: t itself for a scalar.
func (t Type) Elem() Type {
	if t.Many() {
		return t - TypeSeqInt + TypeInt
	}
	return t
}

// Seq is the collection type over t's scalar type.
func (t Type) Seq() Type {
	if !t.Scalar() {
		return t
	}
	return t - TypeInt + TypeSeqInt
}

// Mult is a declared multiplicity, checked on the count of values a collection
// feature is bound to. Upper is negative for an unbounded feature.
type Mult struct {
	Lower, Upper int64
}

// MultAny admits every count; MultOne is `[1]`.
var (
	MultAny = Mult{Lower: 0, Upper: -1}
	MultOne = Mult{Lower: 1, Upper: 1}
)

// Admits reports whether a value count n satisfies m.
func (m Mult) Admits(n int64) bool {
	return n >= m.Lower && (m.Upper < 0 || n <= m.Upper)
}

// Range narrows an Integer to the values its declared library subtype admits.
// The interpreter judges Positive as Natural, so both admit every non-negative
// Integer and differ only in the type the refusal names.
type Range int

const (
	RangeAny Range = iota
	RangeNatural
	RangePositive
)

// String is the library type name the range comes from; empty for RangeAny.
func (r Range) String() string {
	switch r {
	case RangeNatural:
		return "Natural"
	case RangePositive:
		return "Positive"
	}
	return ""
}

// Program is a set of compiled functions with one entry point. Collections
// is set when any function handles a collection, which the emitters' element
// budget and per-statement release then track.
type Program struct {
	Funcs       []*Func
	Entry       *Func
	Collections bool
}

// Func is one compiled calculation.
type Func struct {
	Name   string // qualified SysML name
	Ident  string // identifier valid in every target language
	Params []Param
	Result Type
	// ResultRange is checked on every return; a collection result's
	// multiplicity is checked by the Checked the return wraps.
	ResultRange Range
	Body        []Stmt
}

// Param is one input parameter; Range and, for a collection, Mult and Unique
// are checked on entry.
type Param struct {
	Name   string
	Type   Type
	Range  Range
	Mult   Mult
	Unique bool
}

// Expr is a typed expression.
type Expr interface {
	Type() Type
}

// IntLit, RealLit and BoolLit are literals.
type IntLit struct{ Value int64 }
type RealLit struct{ Value float64 }
type BoolLit struct{ Value bool }

// Var reads a parameter or a body-local variable.
type Var struct {
	Name string
	T    Type
}

// Binary applies an arithmetic, comparison or logical operator. Operands are
// already coerced to a common type; T is the result type.
type Binary struct {
	Op   ast.OperatorKind
	L, R Expr
	T    Type
}

// Unary applies `-`, `+` or `not`.
type Unary struct {
	Op ast.OperatorKind
	X  Expr
	T  Type
}

// Cond is `if c ? a else b`, both branches of type T.
type Cond struct {
	C, Then, Else Expr
	T             Type
}

// Call invokes another compiled function, arguments coerced to parameter types.
// Call invokes Fn. Args are evaluated in source order, each binding the
// parameter at Param; a parameter named twice takes the later value.
type Call struct {
	Fn   *Func
	Args []Arg
}

// Arg is one supplied argument of a Call.
type Arg struct {
	Param int
	Value Expr
}

// LibCall applies a library function operation (library.go); Args bind its
// operands as a Call's do, each coerced to the operand's type.
type LibCall struct {
	Op   LibOp
	Args []Arg
}

// ToReal widens an Integer to a Real; over a collection, every element.
type ToReal struct{ X Expr }

// NullLit is `null`, the empty value of collection type T (or TypeNull).
type NullLit struct{ T Type }

// SeqLit is `(a, b, …)`: the elements of each collection operand, in order,
// as a sequence of type T.
type SeqLit struct {
	Elems []Expr
	T     Type
}

// ToMany views a scalar as the collection holding it, as the interpreter does.
type ToMany struct{ X Expr }

// ToOne is the one scalar X holds; Where names a `[1]` binding, else Fail's %s
// is the shape found ("a sequence", bare "sequence") and Other's a second %s.
type ToOne struct {
	X        Expr
	Where    string
	Fail     string
	Bare     bool
	Other    Expr
	OtherOne string
}

// Checked binds a collection to a feature of multiplicity M and range R at
// Where, refusing a repeated element when Unique, failing as the interpreter's
// binding does.
type Checked struct {
	X      Expr
	M      Mult
	R      Range
	Unique bool
	Where  string
}

// Let evaluates Value into the temporary Name, then In, which reads it as a Var.
type Let struct {
	Name  string
	Value Expr
	In    Expr
}

// Coalesce is `L ?? R`: L unless it is null. Both are of collection type T.
type Coalesce struct {
	L, R Expr
	T    Type
}

// SeqEq is `==` (Neq negated) over collections of one type: same shape, same
// elements in order. Ident is `===`, which no coercion precedes.
type SeqEq struct {
	L, R  Expr
	Neq   bool
	Ident bool
}

// Index is `Seq#(I)`, one-based over the elements of Seq; I is an Integer.
type Index struct {
	Seq, I Expr
}

// RangeExpr is `Lo..Hi`, the Integers from Lo to Hi, empty when Lo > Hi.
type RangeExpr struct{ Lo, Hi Expr }

// SeqCall applies a collection operation (seqops.go) to operands in
// parameter order; T is its result type.
type SeqCall struct {
	Op   SeqOp
	Args []Expr
	T    Type
}

// Lambda is a body expression `{in x; …}`: Params are bound to the
// operation's per-element arguments in order, then Body is evaluated; locals
// the body declares are inlined into Body where it names them.
type Lambda struct {
	Params []Param
	Body   Expr
}

// Fold applies a body operation (seqops.go) over the elements of Seq; T is
// its result type.
type Fold struct {
	Op   SeqOp
	Seq  Expr
	Body Lambda
	T    Type
}

// Framed evaluates X as the inlined body of a library calc: one frame
// deeper against the recursion budget, left once X has answered.
type Framed struct{ X Expr }

// Sampled takes the sample S for the duration of In, which reads S.Dom and
// S.Rng as Vars.
type Sampled struct {
	S  Sample
	In Expr
}

func (IntLit) Type() Type    { return TypeInt }
func (RealLit) Type() Type   { return TypeReal }
func (BoolLit) Type() Type   { return TypeBool }
func (v Var) Type() Type     { return v.T }
func (b Binary) Type() Type  { return b.T }
func (u Unary) Type() Type   { return u.T }
func (c Cond) Type() Type    { return c.T }
func (c Call) Type() Type    { return c.Fn.Result }
func (l LibCall) Type() Type { return l.Op.Result() }
func (t ToReal) Type() Type {
	if t.X.Type().Many() {
		return TypeSeqReal
	}
	return TypeReal
}
func (n NullLit) Type() Type  { return n.T }
func (s SeqLit) Type() Type   { return s.T }
func (t ToMany) Type() Type   { return t.X.Type().Seq() }
func (t ToOne) Type() Type    { return t.X.Type().Elem() }
func (c Checked) Type() Type  { return c.X.Type() }
func (l Let) Type() Type      { return l.In.Type() }
func (c Coalesce) Type() Type { return c.T }
func (SeqEq) Type() Type      { return TypeBool }
func (i Index) Type() Type    { return i.Seq.Type().Elem() }
func (RangeExpr) Type() Type  { return TypeSeqInt }
func (s SeqCall) Type() Type  { return s.T }
func (f Fold) Type() Type     { return f.T }
func (f Framed) Type() Type   { return f.X.Type() }
func (s Sampled) Type() Type  { return s.In.Type() }

// Stmt is a statement of a function body.
type Stmt interface{ stmt() }

// Declare introduces a body-local variable with its initial value, null (a
// collection) when Init is nil. The interpreter judges an initializer's
// uniqueness but not its range or multiplicity, so neither does generated
// code; later assignments are checked in full.
type Declare struct {
	Name string
	T    Type
	Init Expr
}

// Assign writes a body-local variable or parameter; a scalar is checked
// against Range, a collection by the Checked or ToOne that Value is.
type Assign struct {
	Name  string
	Range Range
	Value Expr
}

// If runs Then or Else on a Boolean condition.
type If struct {
	Cond Expr
	Then []Stmt
	Else []Stmt
}

// While runs Body while Cond holds; Until, if set, is tested after each pass
// and stops the loop when it holds.
type While struct {
	Cond  Expr
	Until Expr
	Body  []Stmt
}

// ForEach runs Body once per element of Seq, bound to Var; a bare scalar
// is not iterable and fails, as the interpreter's `for` does.
type ForEach struct {
	Var  string
	Seq  Expr
	Body []Stmt
}

// Sample takes `Sample(f, Seq)` one frame deeper: Dom gets the domain values,
// Rng Body at each in order, every sample charged as the interpreter's pair is.
type Sample struct {
	Dom, Rng string
	Seq      Expr
	Body     Lambda
}

// DomType and RngType are the collection types of Dom and Rng.
func (s Sample) DomType() Type { return s.Seq.Type() }
func (s Sample) RngType() Type { return s.Body.Body.Type().Seq() }

// Return answers the function's result.
type Return struct{ Value Expr }

func (Declare) stmt() { /* marker: Stmt */ }
func (Assign) stmt()  { /* marker: Stmt */ }
func (If) stmt()      { /* marker: Stmt */ }
func (While) stmt()   { /* marker: Stmt */ }
func (ForEach) stmt() { /* marker: Stmt */ }
func (Sample) stmt()  { /* marker: Stmt */ }
func (Return) stmt()  { /* marker: Stmt */ }
