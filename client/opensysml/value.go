package opensysml

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// Value is one evaluated SysML value. It is a sealed sum: the concrete types
// are Int, Real, Complex, Bool, String, InstanceID, Sequence, Null, Unset,
// Quantity, EnumLiteral, Array, Vector, VectorQuantity, MeasurementRef,
// Function, Set, TensorQuantity and Metaobject, and a type switch over them is
// exhaustive.
type Value interface {
	isValue()
}

// Number is a Value that is a numeric magnitude: Int or Real. Integers and
// reals stay apart end to end, so an integer compares exactly.
type Number interface {
	Value
	isNumber()
}

// Int is an integer value.
type Int int64

// Real is a real value.
type Real float64

// Complex is one complex number, never a sequence of two Reals. It is not a
// Number: a quantity's magnitude is a real magnitude.
type Complex complex128

// Bool is a boolean value.
type Bool bool

// String is a string value.
type String string

// InstanceID refers to an Instance by the id an Instantiation assigned it.
// Ids are per call: they name instances within one answer, nothing beyond it.
type InstanceID int64

// Sequence is an ordered collection of values.
type Sequence []Value

// Null is the absence of a value. Its text is the service's reason when it has
// one ("unsupported: variant selection"), and empty for a plain null.
type Null string

// Unset is a valueless feature of a value type: materialized, holding no
// value. It is reported, never accepted.
type Unset struct{}

// Quantity is a magnitude and the measurement unit it is expressed in, exactly
// as written: 5.4 [km/h] arrives as 5.4 km/h, not converted to base units.
type Quantity struct {
	// Magnitude keeps Int and Real apart, as the rest of Value does.
	Magnitude Number
	// Unit as written ("km/h"), or as an operation composed it ("m/s"); empty
	// for a unit never written down, described by Term alone.
	Unit string
	// Term is what the unit reduces to, which decides commensurability.
	Term *UnitTerm
}

// UnitTerm is a unit's reduction to base units, with its scale kept as an
// unevaluated ratio so an exact conversion stays exact.
type UnitTerm struct {
	ScaleNum float64
	ScaleDen float64
	// Factors are the base units of the reduction, carrying no zero exponents.
	Factors []UnitFactor
}

// UnitFactor is one base unit raised to an exponent.
type UnitFactor struct {
	// UnitID is the FQN of the base unit ("SI::m").
	UnitID string
	// Exponent the base unit is raised to.
	Exponent float64
}

// MeasurementRef is a measurement unit held as a value by itself, with no
// magnitude: `SI::m`, `km`, or `m / s` as an operation composed it. It is what
// a MeasurementUnit-typed attribute or a quantity's mRef evaluates to, and what
// ConvertQuantity takes as its target.
type MeasurementRef struct {
	// Unit as written ("km") or as an operation composed it ("m/s"); empty
	// for one never written down, described by Term alone.
	Unit string
	// Term is what the unit reduces to. Required wherever Unit names a unit.
	Term *UnitTerm
	// UnitID is the FQN of the one unit declaration the reference names
	// ("SI::kilometre"); empty for a unit an operation composed, which names none.
	UnitID string
}

// Function is a calc held as a value: a calc definition, a calc usage or an
// `in calc` parameter read where a value is expected. It names the calc and, for
// one read off an object, that object; sent as an argument it is invoked through
// the calc-typed parameter it binds. Identity is the calc together with Self.
type Function struct {
	// CalcID is the FQN of the calc declaration ("M::Sq").
	CalcID string
	// Self is the object the calc computes over, an id of the answer that
	// reported it; 0 for a calc bound to no object. As any InstanceID it lives
	// only within that answer: sent as an argument, a non-zero Self is refused.
	Self InstanceID
}

// EnumLiteral is one literal of an enumeration definition. A literal is its
// own identity: two values are the same literal exactly when LiteralID is.
type EnumLiteral struct {
	// LiteralID is the FQN of the literal's declaration ("D::Color::red").
	LiteralID string
	// EnumerationID is the FQN of the enumeration declaring it ("D::Color").
	EnumerationID string
	// Name is the literal as a reader writes it ("Color::red").
	Name string
	// Value is the scalar the literal equals — Int(3) for `high = 3` of an
	// enumeration specializing Integer — or nil for a literal that is only its
	// identity. It describes the literal and is no part of its identity.
	Value Value
}

// Array is a Collections::Array: its elements in row-major order under its
// dimensions. Its rank is the number of dimensions; an array of rank 0 holds
// one element. Elements are Values, so an array of quantities or of arrays is
// one as such.
type Array struct {
	// Dimensions are the positive extents, one per rank.
	Dimensions []int64
	// Elements fill the dimensions in row-major order.
	Elements []Value
}

// Vector is a numerical vector: its components in order, each an Int or a Real,
// kept apart as the rest of Value does. Its dimension is the number of them.
type Vector []Number

// VectorQuantity is a vector quantity: one Quantity per axis, unit and
// reduction included, since the axes need not share a unit.
type VectorQuantity []Quantity

// Set is a unique, unordered collection — a Collections::Set's elements — as
// distinct from a Sequence, whose order is part of its value. The service
// sends the elements in its canonical order, so equal sets arrive alike; a
// caller may list them in any order, but one listing an element twice, by
// Equal, is refused when sent rather than read as one element.
type Set []Value

// TensorQuantity is a tensor quantity of any rank: one Quantity per component,
// unit and reduction included, in row-major order under its dimensions. A
// tensor of rank one is not a VectorQuantity, here as in the runtime.
type TensorQuantity struct {
	// Dimensions are the positive extents, one per rank.
	Dimensions []int64
	// Components fill the dimensions in row-major order.
	Components []Quantity
}

// String renders the quantity as it was written: magnitude then unit.
func (q Quantity) String() string {
	magnitude := fmt.Sprintf("%v", q.Magnitude)
	if q.Unit == "" {
		return magnitude
	}
	return magnitude + " " + q.Unit
}

// String renders the number in rectangular form as SysML writes it: `1.5 - 2.0i`.
func (z Complex) String() string {
	return runtime.FormatComplex(complex128(z))
}

// String renders the array as SysML formats it: `Array(2, 2)[1, 2, 3, 4]`.
func (a Array) String() string {
	dims := make([]string, len(a.Dimensions))
	for i, d := range a.Dimensions {
		dims[i] = fmt.Sprintf("%d", d)
	}
	elements := make([]string, len(a.Elements))
	for i, e := range a.Elements {
		elements[i] = fmt.Sprintf("%v", e)
	}
	return "Array(" + strings.Join(dims, ", ") + ")[" + strings.Join(elements, ", ") + "]"
}

// String renders the vector as SysML formats it: `⟨3.0, 4.0⟩`.
func (v Vector) String() string {
	parts := make([]string, len(v))
	for i, c := range v {
		parts[i] = fmt.Sprintf("%v", c)
	}
	return "⟨" + strings.Join(parts, ", ") + "⟩"
}

// String renders the vector quantity as SysML formats it: `⟨3.0, 4.0⟩ m` when
// every axis shares a unit, else each component with its own.
func (vq VectorQuantity) String() string {
	shared := len(vq) > 0
	for _, q := range vq[min(1, len(vq)):] {
		shared = shared && q.Unit == vq[0].Unit
	}
	parts := make([]string, len(vq))
	for i, q := range vq {
		if shared {
			parts[i] = fmt.Sprintf("%v", q.Magnitude)
		} else {
			parts[i] = q.String()
		}
	}
	out := "⟨" + strings.Join(parts, ", ") + "⟩"
	if shared && vq[0].Unit != "" {
		out += " " + vq[0].Unit
	}
	return out
}

// String renders the set as SysML traces format it: `{1, 2, 3}`, the elements
// in the order held.
func (s Set) String() string {
	parts := make([]string, len(s))
	for i, e := range s {
		parts[i] = fmt.Sprintf("%v", e)
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// String renders the tensor as SysML formats it: `Tensor(2, 2, 2)[1.0, 2.0, ...] m`
// when every component shares a unit, else each component with its own.
func (tq TensorQuantity) String() string {
	dims := make([]string, len(tq.Dimensions))
	for i, d := range tq.Dimensions {
		dims[i] = fmt.Sprintf("%d", d)
	}
	shared := len(tq.Components) > 0
	for _, q := range tq.Components[min(1, len(tq.Components)):] {
		shared = shared && q.Unit == tq.Components[0].Unit
	}
	parts := make([]string, len(tq.Components))
	for i, q := range tq.Components {
		if shared {
			parts[i] = fmt.Sprintf("%v", q.Magnitude)
		} else {
			parts[i] = q.String()
		}
	}
	out := "Tensor(" + strings.Join(dims, ", ") + ")[" + strings.Join(parts, ", ") + "]"
	if shared && tq.Components[0].Unit != "" {
		out += " " + tq.Components[0].Unit
	}
	return out
}

// String renders the reference as SysML writes the unit: `km`, `m/s`; one
// never written down renders its reduction.
func (r MeasurementRef) String() string {
	if r.Unit != "" || r.Term == nil {
		return r.Unit
	}
	return r.Term.String()
}

// String renders the reduction over its base units, `1000/3600·SI::m·SI::s^-1`,
// a scale of one and an exponent of one left implicit; `1` for dimension one.
func (t UnitTerm) String() string {
	var parts []string
	if t.ScaleNum != t.ScaleDen {
		parts = append(parts, fmt.Sprintf("%g/%g", t.ScaleNum, t.ScaleDen))
	}
	for _, f := range t.Factors {
		if f.Exponent == 1 {
			parts = append(parts, f.UnitID)
			continue
		}
		parts = append(parts, fmt.Sprintf("%s^%g", f.UnitID, f.Exponent))
	}
	if len(parts) == 0 {
		return "1"
	}
	return strings.Join(parts, "·")
}

// String names the calc the function is a value of.
func (f Function) String() string {
	return f.CalcID
}

// Metaobject is an element of the model held as an instance of its reflective
// metaclass: what `x meta KerML::Feature`, or the last element of `x.metadata`,
// evaluates to. Identity is the element: two metaobjects are the same exactly
// when ElementID is. Its features are read in the model, not carried.
type Metaobject struct {
	// ElementID is the FQN of the element reflected on ("Vehicle::seatBelt").
	ElementID string
	// MetaclassID is the FQN of the element's own reflective metaclass
	// ("SysML::Systems::PartUsage"), not the type it was cast to. The service
	// always reports it; a caller may leave it empty to have the model's used.
	MetaclassID string
}

// String is the metaobject as the REPL shows it: the element and its metaclass.
func (m Metaobject) String() string {
	return "meta(" + m.ElementID + " : " + m.MetaclassID + ")"
}

// String is the literal as a reader writes it, the Name the service reported.
func (e EnumLiteral) String() string {
	return e.Name
}

// String reports the absence of a value, with the service's reason when it
// gave one.
func (n Null) String() string {
	if n == "" {
		return "null"
	}
	return "null: " + string(n)
}

// String reports a materialized feature holding no value.
func (Unset) String() string {
	return "unset"
}

func (Int) isValue()            { /* marker: closed Value set */ }
func (Real) isValue()           { /* marker: closed Value set */ }
func (Complex) isValue()        { /* marker: closed Value set */ }
func (Bool) isValue()           { /* marker: closed Value set */ }
func (String) isValue()         { /* marker: closed Value set */ }
func (InstanceID) isValue()     { /* marker: closed Value set */ }
func (Sequence) isValue()       { /* marker: closed Value set */ }
func (Null) isValue()           { /* marker: closed Value set */ }
func (Unset) isValue()          { /* marker: closed Value set */ }
func (Quantity) isValue()       { /* marker: closed Value set */ }
func (EnumLiteral) isValue()    { /* marker: closed Value set */ }
func (Array) isValue()          { /* marker: closed Value set */ }
func (Vector) isValue()         { /* marker: closed Value set */ }
func (VectorQuantity) isValue() { /* marker: closed Value set */ }
func (MeasurementRef) isValue() { /* marker: closed Value set */ }
func (Function) isValue()       { /* marker: closed Value set */ }
func (Set) isValue()            { /* marker: closed Value set */ }
func (TensorQuantity) isValue() { /* marker: closed Value set */ }
func (Metaobject) isValue()     { /* marker: closed Value set */ }

func (Int) isNumber()  { /* marker: closed Number set */ }
func (Real) isNumber() { /* marker: closed Number set */ }
