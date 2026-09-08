package runtime

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ValueKind distinguishes runtime value types.
type ValueKind int

const (
	ValInvalid ValueKind = iota
	ValConst             // wraps semantics.Value (int/real/bool/infinity)
	ValNull
	ValString
	ValInstance
	ValSequence
	ValSet
	ValExpr                     // wraps unevaluated AST node for delayed evaluation (e.g., BodyExpr for select/collect)
	ValQuantity                 // a magnitude and the measurement unit it is expressed in
	ValVariant                  // the variant selected for a variation, and the object it materializes
	ValEnumLiteral              // one literal of an enumeration definition, identified by itself
	ValComplex                  // one complex number, its real and imaginary parts together
	ValArray                    // a Collections::Array: its dimensions and its row-major elements
	ValVector                   // a NumericalVectorValue: the numbers of its one dimension
	ValVectorQuantity           // a VectorQuantityValue: a vector with a measurement unit per axis
	ValMeasurementRef           // a ScalarMeasurementReference: a unit by declaration and reduction
	ValTensorQuantity           // a TensorQuantityValue: an array of numbers with a measurement unit per component
	ValCoordinateFrame          // a VectorMeasurementReference: a frame's axes, or a measurement scale's one
	ValCoordinateTransformation // a CoordinateTransformation: a placement of one frame in another
	ValFunction                 // a calc as a value: its lowered shape closed over the environment it was read in

	// valueKindCount bounds the kinds; TestEveryValueKindIsDispatched walks them.
	valueKindCount
)

// unknownText stands for a value or reference that is absent where a name is rendered.
const unknownText = "<unknown>"

// FormatValue renders a value with the notation used by user-facing runtime
// results and diagnostics.
func FormatValue(v Value) string {
	switch v.Kind {
	case ValConst:
		return semantics.FormatConst(v.Const)
	case ValNull:
		return "null"
	case ValString:
		return strconv.Quote(v.Str())
	case ValInstance:
		return fmt.Sprintf("instance(%d)", v.Instance)
	case ValSequence:
		seq := v.Sequence()
		if seq == nil {
			return "[]"
		}
		return "[" + strings.Join(formatValueElements(seq.Elements()), ", ") + "]"
	case ValSet:
		set := v.Set()
		if set == nil {
			return "Set{}"
		}
		parts := formatValueElements(set.Elements())
		return "Set{" + strings.Join(parts, ", ") + "}"
	case ValVariant:
		variant := v.Variant()
		if variant == nil {
			return "<unknown variant>"
		}
		if v.Instance != 0 {
			return fmt.Sprintf("%s (Instance ID: %d)", variant.Name, v.Instance)
		}
		return variant.Name
	case ValEnumLiteral:
		return v.LiteralText()
	case ValQuantity:
		q := v.Quantity()
		if q == nil {
			return unknownText
		}
		return q.TextWithMagnitude(semantics.FormatConst(q.Num))
	case ValComplex:
		return FormatComplex(v.Complex())
	case ValArray:
		return v.Array().Format(FormatValue)
	case ValVector:
		return v.Vector().format(semantics.FormatConst)
	case ValVectorQuantity:
		return v.VectorQuantity().format(semantics.FormatConst)
	case ValMeasurementRef:
		return v.MeasurementRef().String()
	case ValTensorQuantity:
		return v.TensorQuantity().format(semantics.FormatConst)
	case ValCoordinateFrame:
		return v.CoordinateFrame().String()
	case ValCoordinateTransformation:
		return v.CoordinateTransformation().String()
	case ValExpr:
		return "<expression>"
	case ValFunction:
		return v.FunctionName()
	default:
		return unknownText
	}
}

// FormatComplex renders a complex number as its parts read, `1.0 + 2.0i`, with
// the imaginary part's sign as the operator: `1.0 - 2.0i`.
func FormatComplex(z complex128) string {
	re, im := real(z), imag(z)
	sign := " + "
	if math.Signbit(im) {
		sign, im = " - ", -im
	}
	return semantics.FormatReal(re) + sign + semantics.FormatReal(im) + "i"
}

func formatValueElements(elements []Value) []string {
	parts := make([]string, len(elements))
	for i, element := range elements {
		parts[i] = FormatValue(element)
	}
	return parts
}

// String names the kind, so diagnostics quoting it read as more than an index.
func (k ValueKind) String() string {
	switch k {
	case ValConst:
		return "constant"
	case ValNull:
		return "null"
	case ValString:
		return "string"
	case ValInstance:
		return "instance"
	case ValSequence:
		return "sequence"
	case ValSet:
		return "set"
	case ValExpr:
		return "expression"
	case ValQuantity:
		return "quantity"
	case ValVariant:
		return "variant"
	case ValEnumLiteral:
		return "enumeration literal"
	case ValComplex:
		return "complex number"
	case ValArray:
		return "array"
	case ValVector:
		return "vector"
	case ValVectorQuantity:
		return "vector quantity"
	case ValMeasurementRef:
		return "measurement reference"
	case ValTensorQuantity:
		return "tensor quantity"
	case ValCoordinateFrame:
		return "coordinate frame"
	case ValCoordinateTransformation:
		return "coordinate transformation"
	case ValFunction:
		return "function"
	default:
		return "invalid"
	}
}

// Value is a runtime-evaluable value. The scalar payload stays inline because
// arithmetic copies values through every evaluator frame; the rarer payloads
// share one slot so the struct stays at 64 bytes.
type Value struct {
	Kind     ValueKind
	Const    semantics.Value // ValConst: reuse static evaluator
	Instance int64           // ValInstance: instance ID; ValVariant: materialized object, 0 for none
	// ref holds the kind-specific payload of the remaining kinds: a string
	// (ValString), *Sequence, *Set, *exprValue (ValExpr), *Quantity, a complex128
	// (ValComplex), *Array, *Vector, *VectorQuantity, *MeasurementRef, *TensorQuantity,
	// *functionValue (ValFunction), or the *symbols.Symbol of a variant (ValVariant) or
	// enumeration literal (ValEnumLiteral).
	ref any
}

// NewComplex is the value of one complex number. One with a zero imaginary part
// is the Real its real part is, as 4 / 2 is the Integer 2: equal to it, and
// classified as it is.
func NewComplex(z complex128) Value {
	return Value{Kind: ValComplex, ref: z}
}

// NewStringValue is the value of a string.
func NewStringValue(s string) Value {
	return Value{Kind: ValString, ref: s}
}

// NewSequenceValue wraps an ordered collection. A nil sequence is the empty one.
func NewSequenceValue(seq *Sequence) Value {
	return Value{Kind: ValSequence, ref: seq}
}

// NewSetValue wraps a unique collection. A nil set is the empty one.
func NewSetValue(set *Set) Value {
	return Value{Kind: ValSet, ref: set}
}

// exprValue is a deferred expression closed over the environment it was
// written in; a nil env evaluates it where it is applied.
type exprValue struct {
	node ast.Node
	env  *EvalContext
}

// NewExprValue defers evaluation of an expression, closing over env: the
// bindings and scope in force where it was written.
func NewExprValue(node ast.Node, env *EvalContext) Value {
	return Value{Kind: ValExpr, ref: &exprValue{node: node, env: env}}
}

// NewQuantityValue wraps a magnitude expressed in a measurement unit.
func NewQuantityValue(q *Quantity) Value {
	return Value{Kind: ValQuantity, ref: q}
}

// NewVariantValue is the variant a variation was bound to, with the object it
// materialized (0 when it materializes none).
func NewVariantValue(variant *symbols.Symbol, instance int64) Value {
	return Value{Kind: ValVariant, ref: variant, Instance: instance}
}

// NewEnumLiteral is the value an enumeration literal that declares no value of
// its own evaluates to: the identity of that literal.
func NewEnumLiteral(sym *symbols.Symbol) Value {
	return Value{Kind: ValEnumLiteral, ref: sym}
}

// Str is the text of a ValString; "" for every other kind.
func (v Value) Str() string {
	if v.Kind != ValString {
		return ""
	}
	s, _ := v.ref.(string)
	return s
}

// Sequence is the collection of a ValSequence; nil for every other kind.
func (v Value) Sequence() *Sequence {
	if v.Kind != ValSequence {
		return nil
	}
	seq, _ := v.ref.(*Sequence)
	return seq
}

// Set is the collection of a ValSet; nil for every other kind.
func (v Value) Set() *Set {
	if v.Kind != ValSet {
		return nil
	}
	set, _ := v.ref.(*Set)
	return set
}

// Expr is the deferred expression of a ValExpr; nil for every other kind.
func (v Value) Expr() ast.Node {
	if v.Kind != ValExpr {
		return nil
	}
	if closure, ok := v.ref.(*exprValue); ok {
		return closure.node
	}
	return nil
}

// exprEnv is the environment a ValExpr closes over, or in where it closes over
// none. Tracing is the applying context's: a body applied later is recorded
// as the evaluation reaching it is, not as its creation was.
func (v Value) exprEnv(in *EvalContext) *EvalContext {
	closure, ok := v.ref.(*exprValue)
	if !ok || v.Kind != ValExpr || closure.env == nil {
		return in
	}
	if closure.env.trace == in.trace {
		return closure.env
	}
	env := *closure.env
	env.trace = in.trace
	return &env
}

// Quantity is the payload of a ValQuantity; nil for every other kind.
func (v Value) Quantity() *Quantity {
	if v.Kind != ValQuantity {
		return nil
	}
	q, _ := v.ref.(*Quantity)
	return q
}

// Array is the payload of a ValArray; nil for every other kind.
func (v Value) Array() *Array {
	if v.Kind != ValArray {
		return nil
	}
	a, _ := v.ref.(*Array)
	return a
}

// Vector is the payload of a ValVector; nil for every other kind.
func (v Value) Vector() *Vector {
	if v.Kind != ValVector {
		return nil
	}
	vec, _ := v.ref.(*Vector)
	return vec
}

// VectorQuantity is the payload of a ValVectorQuantity; nil for every other kind.
func (v Value) VectorQuantity() *VectorQuantity {
	if v.Kind != ValVectorQuantity {
		return nil
	}
	vq, _ := v.ref.(*VectorQuantity)
	return vq
}

// MeasurementRef is the payload of a ValMeasurementRef; nil for every other kind.
func (v Value) MeasurementRef() *MeasurementRef {
	if v.Kind != ValMeasurementRef {
		return nil
	}
	ref, _ := v.ref.(*MeasurementRef)
	return ref
}

// TensorQuantity is the payload of a ValTensorQuantity; nil for every other kind.
func (v Value) TensorQuantity() *TensorQuantity {
	if v.Kind != ValTensorQuantity {
		return nil
	}
	tq, _ := v.ref.(*TensorQuantity)
	return tq
}

// CoordinateFrame is the payload of a ValCoordinateFrame; nil for every other kind.
func (v Value) CoordinateFrame() *CoordinateFrame {
	if v.Kind != ValCoordinateFrame {
		return nil
	}
	frame, _ := v.ref.(*CoordinateFrame)
	return frame
}

// CoordinateTransformation is the payload of a ValCoordinateTransformation; nil
// for every other kind.
func (v Value) CoordinateTransformation() *CoordinateTransformation {
	if v.Kind != ValCoordinateTransformation {
		return nil
	}
	t, _ := v.ref.(*CoordinateTransformation)
	return t
}

// Complex is the number a ValComplex is; 0 for every other kind.
func (v Value) Complex() complex128 {
	if v.Kind != ValComplex {
		return 0
	}
	z, _ := v.ref.(complex128)
	return z
}

// Variant is the variant a ValVariant was bound to; nil for every other kind.
func (v Value) Variant() *symbols.Symbol {
	if v.Kind != ValVariant {
		return nil
	}
	sym, _ := v.ref.(*symbols.Symbol)
	return sym
}

// Literal is the enumeration literal a ValEnumLiteral is: a literal is its own
// identity, so two values are the same literal exactly when they name the same
// declaration. Nil for every other kind.
func (v Value) Literal() *symbols.Symbol {
	if v.Kind != ValEnumLiteral {
		return nil
	}
	sym, _ := v.ref.(*symbols.Symbol)
	return sym
}

// LiteralText renders an enumeration literal as it is written, qualified by the
// enumeration it is a literal of: `Color::red`.
func (v Value) LiteralText() string {
	lit := v.Literal()
	if lit == nil {
		return "<unknown enumeration literal>"
	}
	if enum := semantics.EnumerationOwning(lit); enum != nil {
		return enum.Name + "::" + lit.Name
	}
	return lit.Name
}

// Object returns the object a value denotes: an instance, or the object a
// selected variant materialized.
func (v Value) Object() (int64, bool) {
	switch v.Kind {
	case ValInstance:
		return v.Instance, true
	case ValVariant:
		return v.Instance, v.Instance != 0
	default:
		return 0, false
	}
}

// Sequence is an ordered collection (slice-backed). One read empty from a
// quantity-typed declaration remembers the unit its elements would measure in.
type Sequence struct {
	elements    []Value
	elementUnit *Unit
}

// NewSequence creates an empty Sequence.
func NewSequence() *Sequence {
	return &Sequence{elements: make([]Value, 0)}
}

// NewEmptySequenceOf is the empty sequence of quantities measured in unit, which
// types the identity an aggregate of it yields.
func NewEmptySequenceOf(unit Unit) Value {
	return NewSequenceValue(&Sequence{elements: make([]Value, 0), elementUnit: &unit})
}

// ElementUnit is the unit an empty sequence's elements are declared in; false for
// a sequence holding elements or read from no quantity-typed declaration.
func (s *Sequence) ElementUnit() (Unit, bool) {
	if s == nil || s.elementUnit == nil || len(s.elements) != 0 {
		return Unit{}, false
	}
	return *s.elementUnit, true
}

// Append adds a value to the end of the sequence.
func (s *Sequence) Append(val Value) {
	s.elements = append(s.elements, val)
}

// At returns the element at the given index (0-based).
func (s *Sequence) At(index int) (Value, error) {
	if index < 0 || index >= len(s.elements) {
		return Value{}, fmt.Errorf("index %d out of range [0, %d)", index, len(s.elements))
	}
	return s.elements[index], nil
}

// Size returns the number of elements.
func (s *Sequence) Size() int {
	return len(s.elements)
}

// Elements returns the underlying slice (for iteration).
func (s *Sequence) Elements() []Value {
	return s.elements
}

// Set is a unique collection backed by hash buckets and exact comparisons. A set
// has no inherent order, but enumerating one has to answer in some order, and
// insertion order is the one order a set does carry: it makes a sequence
// derived from a set — what `select` and `collect` over a set return —
// reproducible instead of dependent on map iteration.
type Set struct {
	elements map[valueKey][]Value
	order    []Value
	size     int
}

// NewSet creates an empty Set.
func NewSet() *Set {
	return &Set{elements: make(map[valueKey][]Value)}
}

// Add inserts a value into the set (deduplicates by exact value equality).
func (s *Set) Add(val Value) {
	key := valueKeyFunc(val)
	bucket := s.elements[key]
	for _, elem := range bucket {
		if valueEqual(elem, val) {
			return
		}
	}
	s.elements[key] = append(bucket, val)
	s.order = append(s.order, val)
	s.size++
}

// Contains checks if the value is in the set.
func (s *Set) Contains(val Value) bool {
	for _, elem := range s.elements[valueKeyFunc(val)] {
		if valueEqual(elem, val) {
			return true
		}
	}
	return false
}

// Size returns the number of unique elements.
func (s *Set) Size() int {
	return s.size
}

// Elements returns all elements, in the order they were added.
func (s *Set) Elements() []Value {
	return append([]Value(nil), s.order...)
}
