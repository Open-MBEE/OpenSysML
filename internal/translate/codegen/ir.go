// Package codegen compiles the value subset of SysML v2 calcs to native code;
// see docs/project/native-compilation.md for the subset and its semantics.
package codegen

import (
	"math/big"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Type is a type of the compiled subset: a scalar, or a collection of scalars.
// A collection value is the interpreter's dynamic view of a multi-valued
// feature: null, one bare scalar, or a sequence of any length (its shape).
// An enumeration-literal type names its enumeration in Enum, a function
// type the functions its values range over in Fns, a record type its
// attribute definition in Rec.
type Type struct {
	k    typeKind
	many bool
	// unset marks a scalar that may be a materialized unset value instead.
	unset bool
	Enum  *Enum
	Fns   *FnSet
	Rec   *Record
}

type typeKind uint8

const (
	kindInvalid typeKind = iota
	kindInt
	kindReal
	kindBool
	kindNum
	kindString
	kindEnum
	kindFunc
	kindRec
	kindNull
	kindRun
)

var (
	TypeInvalid   = Type{}
	TypeInt       = Type{k: kindInt}                // Integer and its subtypes, unbounded (int64 until a result leaves it)
	TypeReal      = Type{k: kindReal}               // Real and Rational, IEEE 754 binary64
	TypeBool      = Type{k: kindBool}               // Boolean
	TypeNum       = Type{k: kindNum}                // a Real-typed value, an Integer or a Real by run-time kind
	TypeString    = Type{k: kindString}             // a String, its characters Unicode code points held as UTF-8
	TypeNull      = Type{k: kindNull}               // `null` before context fixes its collection type
	TypeRun       = Type{k: kindRun}                // the identity of one run of a calc body, which its closures carry
	TypeSeqInt    = Type{k: kindInt, many: true}    // collection of Integers
	TypeSeqReal   = Type{k: kindReal, many: true}   // collection of Reals
	TypeSeqBool   = Type{k: kindBool, many: true}   // collection of Booleans
	TypeSeqNum    = Type{k: kindNum, many: true}    // collection of numbers, each element of its own kind
	TypeSeqString = Type{k: kindString, many: true} // collection of Strings
)

// EnumType is the type of e's literals.
func EnumType(e *Enum) Type { return Type{k: kindEnum, Enum: e} }

// RecType is the type of r's values.
func RecType(r *Record) Type { return Type{k: kindRec, Rec: r} }

// Enum is an enumeration definition whose literals are identified by
// themselves: literal i is the value Base+i of the program's literal table.
type Enum struct {
	Name     string   // qualified name
	Short    string   // the enumeration's own name, which a literal prints under
	Literals []string // literal names in declaration order
	Base     int
	ID       int // position in Program.Enums
}

// Literal is the text the interpreter prints literal i as: `Color::red`.
func (e *Enum) Literal(i int) string { return e.Short + "::" + e.Literals[i] }

// Record is an attribute definition whose values are data values with
// features, each identified by itself as the interpreter's objects are.
type Record struct {
	Name   string // qualified name
	Short  string // the definition's own name, which diagnostics name it by
	Fields []Field
	ID     int // position in Program.Records
	sym    *symbols.Symbol
	pass   int
	why    string
}

// Field is one feature of a record, in the definition's shape order.
type Field struct {
	Name string
	T    Type
	b    binding
	// def is the constant default the feature holds when a constructor binds none.
	def Expr
}

// Field is the index of the feature named name, or -1.
func (r *Record) Field(name string) int {
	for i, f := range r.Fields {
		if f.Name == name {
			return i
		}
	}
	return -1
}

func (t Type) String() string {
	if t.many {
		return t.Elem().String() + "[0..*]"
	}
	switch t.k {
	case kindInt:
		return "Integer"
	case kindReal, kindNum:
		return "Real"
	case kindBool:
		return "Boolean"
	case kindString:
		return "String"
	case kindEnum:
		return t.Enum.Name
	case kindFunc:
		return "function"
	case kindRec:
		return t.Rec.Name
	case kindNull:
		return "null"
	case kindRun:
		return "run"
	}
	return "invalid"
}

// elemKind reports whether k is the kind of a value a collection may hold.
func elemKind(k typeKind) bool {
	switch k {
	case kindInt, kindReal, kindBool, kindNum, kindString, kindEnum, kindFunc, kindRec:
		return true
	}
	return false
}

// Scalar reports whether t is exactly one Integer, Real, Boolean, number,
// String, enumeration literal, function or record.
func (t Type) Scalar() bool { return !t.many && !t.unset && elemKind(t.k) }

// MayUnset reports whether t is a scalar that may instead be unset: the
// interpreter's materialized value of a required feature nothing was written
// to, which counts as one value, has an identity, and fails where a concrete
// value is required.
func (t Type) MayUnset() bool { return t.unset }

// Unsettable is the scalar t admitting an unset value too.
func (t Type) Unsettable() Type {
	t.unset = true
	return t
}

// Concrete is t without an unset value.
func (t Type) Concrete() Type {
	t.unset = false
	return t
}

// IsEnum reports whether t's values are enumeration literals.
func (t Type) IsEnum() bool { return t.k == kindEnum }

// IsFn reports whether t's values are functions.
func (t Type) IsFn() bool { return t.k == kindFunc }

// IsRec reports whether t's values are records.
func (t Type) IsRec() bool { return t.k == kindRec }

// numeric reports whether t's values are Integers, Reals or numbers.
func numeric(t Type) bool {
	e := t.Elem()
	return e == TypeInt || e == TypeReal || e == TypeNum
}

// Many reports whether t is a collection type.
func (t Type) Many() bool { return t.many }

// Elem is the scalar type of t's values: t itself for a scalar.
func (t Type) Elem() Type {
	t.many, t.unset = false, false
	return t
}

// Seq is the collection type over t's scalar type.
func (t Type) Seq() Type {
	if !t.many && elemKind(t.k) {
		t.many, t.unset = true, false
	}
	return t
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

// Range narrows an Integer to the values its declared library subtype admits:
// Natural the non-negative Integers, Positive those above zero.
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

// Lower is the least Integer the range admits.
func (r Range) Lower() int64 {
	if r == RangePositive {
		return 1
	}
	return 0
}

// Program is a set of compiled functions with one entry point. Collections
// is set when any function handles a collection, which the emitters' element
// budget and per-statement release then track.
type Program struct {
	Funcs       []*Func
	Enums       []*Enum
	Records     []*Record
	FnCases     []*FnCase
	Entry       *Func
	Collections bool
	// Target is the backend the program was compiled for.
	Target Target
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
	// Captured is the count of trailing Params a closure's value carries, the
	// bindings of its enclosing body it reads.
	Captured int
	// Run names the variable holding this run's identity, which the closures
	// the body declares carry; empty when none is read.
	Run string
	// self is the record a record's calc is compiled against, held by its
	// last parameter.
	self *Record
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

// IntLit, RealLit and BoolLit are literals. An IntLit beyond int64 holds its
// value in Big, Value then being zero.
type IntLit struct {
	Value int64
	Big   *big.Int
}
type RealLit struct{ Value float64 }
type BoolLit struct{ Value bool }

// StrLit is a String literal, Value its characters as UTF-8.
type StrLit struct{ Value string }

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

// EnumLit is literal I of the enumeration T names, identified by itself.
type EnumLit struct {
	T Type
	I int
}

// EnumText is the qualified name an enumeration literal prints as, the String
// BaseFunctions::ToString gives of it.
type EnumText struct{ X Expr }

// Refusal evaluates Operands in order, then fails. Its message is Parts, joined
// by the interpreter's description of each operand when Describe is set.
type Refusal struct {
	Operands []Expr
	Parts    []string
	Describe bool
	T        Type
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

// ToReal widens an Integer, or a number of either kind, to a Real; over a
// collection, every element.
type ToReal struct{ X Expr }

// ToNum views an Integer or a Real as a number keeping its kind; over a
// collection, every element.
type ToNum struct{ X Expr }

// AsInt is the Integer a number holds, read where NumSplit has found one.
type AsInt struct{ X Expr }

// NumSplit is Int when every number in Nums holds an Integer, else Real; both
// branches are of type T and read only Vars already evaluated.
type NumSplit struct {
	Nums      []Var
	Int, Real Expr
	T         Type
}

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

// Narrowed is the scalar X checked against the range R of the feature it is
// written to.
type Narrowed struct {
	X     Expr
	R     Range
	Where string
}

func (x Narrowed) Type() Type { return x.X.Type() }

// RecNew is a new record of Rec, its features bound to Fields in field
// order, each already evaluated and checked.
type RecNew struct {
	Rec    *Record
	Fields []Expr
}

func (x RecNew) Type() Type { return RecType(x.Rec) }

// RecGet reads feature Field of the record X.
type RecGet struct {
	X     Expr
	Field int
	T     Type
}

func (x RecGet) Type() Type { return x.T }

// NewUnset is a fresh unset value of the scalar type T, which MayUnset, of a
// feature whose Integer values R narrows.
type NewUnset struct {
	T Type
	R Range
}

func (x NewUnset) Type() Type { return x.T }

// Lift is the concrete scalar X as a value of T, its type admitting unset.
type Lift struct {
	X Expr
	T Type
}

func (x Lift) Type() Type { return x.T }

// Need is the value X holds, failing with Fail when X is unset.
type Need struct {
	X    Expr
	Fail string
}

func (x Need) Type() Type { return x.X.Type().Concrete() }

// Strip is the value X holds, which the program has found is not unset.
type Strip struct{ X Expr }

func (x Strip) Type() Type { return x.X.Type().Concrete() }

// Relabel is the concrete V as a value of T that is unset exactly when Of
// is, keeping Of's identity; Of is a Var.
type Relabel struct {
	V, Of Expr
	T     Type
}

func (x Relabel) Type() Type { return x.T }

// IsUnset reports whether X, a Var, is unset.
type IsUnset struct{ X Expr }

func (x IsUnset) Type() Type { return TypeBool }

// SameUnset reports, Neq negated, whether L and R, Vars of which one is
// unset, are the same value.
type SameUnset struct {
	L, R Expr
	Neq  bool
}

func (x SameUnset) Type() Type { return TypeBool }

// Named is X, which may be unset, under the text the interpreter names its
// expression by when it holds no value.
type Named struct {
	X       Expr
	Feature string
}

func (x Named) Type() Type { return x.X.Type() }

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

// RangeExpr is `Lo..Hi`, the Integers from Lo to Hi, empty when Lo > Hi; each
// element spends a step, then is charged to the element budget.
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

// Fold applies a body operation (seqops.go) over the elements of Seq, spending
// Steps once Seq is evaluated; T is its result type.
type Fold struct {
	Op    SeqOp
	Seq   Expr
	Steps int64
	Body  Lambda
	T     Type
}

// Framed evaluates X as the inlined body of a library calc: one frame
// deeper against the recursion budget, left once X has answered.
type Framed struct{ X Expr }

// Steps spends N evaluation steps of the run's step budget, then evaluates X.
type Steps struct {
	N int64
	X Expr
}

// Sampled takes the sample S for the duration of In, which reads S.Dom and
// S.Rng as Vars.
type Sampled struct {
	S  Sample
	In Expr
}

func (IntLit) Type() Type    { return TypeInt }
func (RealLit) Type() Type   { return TypeReal }
func (BoolLit) Type() Type   { return TypeBool }
func (StrLit) Type() Type    { return TypeString }
func (v Var) Type() Type     { return v.T }
func (e EnumLit) Type() Type { return e.T }
func (EnumText) Type() Type  { return TypeString }
func (r Refusal) Type() Type { return r.T }
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
func (t ToNum) Type() Type {
	if t.X.Type().Many() {
		return TypeSeqNum
	}
	return TypeNum
}
func (AsInt) Type() Type      { return TypeInt }
func (n NumSplit) Type() Type { return n.T }
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
func (s Steps) Type() Type    { return s.X.Type() }

// Stmt is a statement of a function body.
type Stmt interface{ stmt() }

// Declare introduces a body-local variable with its initial value, null (a
// collection) when Init is nil. A scalar Init is checked against Range, a
// collection by the Checked or ToOne that Init is, as an Assign is.
type Declare struct {
	Name  string
	T     Type
	Range Range
	Init  Expr
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
// and stops the loop when it holds. Each pass spends a step before Cond.
type While struct {
	Cond  Expr
	Until Expr
	Body  []Stmt
}

// ForEach runs Body once per element of Seq, bound to Var, each pass spending
// a step; a bare scalar is not iterable and fails, as the interpreter's `for` does.
type ForEach struct {
	Var  string
	Seq  Expr
	Body []Stmt
}

// Sample takes `Sample(f, Seq)` one frame deeper: Dom gets the domain values,
// Rng Body at each in order, every sample charged as the interpreter's pair is.
// Steps are spent on entering the frame, before and after Body at each
// element, and once all are taken.
type Sample struct {
	Dom, Rng string
	Seq      Expr
	Body     Lambda
	Steps    SampleSteps
}

// SampleSteps are the steps a Sample spends at each point of its library body.
type SampleSteps struct {
	Enter, Before, After, Done int64
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
