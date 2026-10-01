// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

// This file is protoconv's value conversion restated over the engine's JSON
// messages: every arm of Value in both directions, with the helpers each arm
// needs. Behavior matches a default-capabilities service, so no arm is ever
// filtered to a capability null.

package engine

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/objref"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// boundText renders a multiplicity bound; an unevaluable one renders empty.
func boundText(b semantics.Bound) string {
	if !b.Known {
		return ""
	}
	if b.Infinite {
		return "*"
	}
	return strconv.FormatInt(b.Value, 10)
}

func i64(n int64) *I64      { v := I64(n); return &v }
func f64(f float64) *F64    { v := F64(f); return &v }
func boolean(b bool) *bool  { v := b; return &v }
func text(s string) *string { v := s; return &v }

// valueToProto converts a value read from a live context, which is what tells
// an object holding no value from one with features: the former crosses as the
// unset arm, as every other surface reads it. rt may be nil for a value no
// context materialized.
func valueToProto(rt *runtime.Context, val runtime.Value, idx *symbols.Index) *JValue {
	if rt != nil && rt.HoldsNoValue(val) {
		return &JValue{Unset: boolean(true)}
	}
	if val.EnumerationLiteral() != nil {
		return enumLiteralValueToProto(val, idx)
	}
	switch val.Kind {
	case runtime.ValConst:
		switch val.Const.Kind {
		case semantics.ValInt:
			return &JValue{IntValue: i64(val.Const.Int)}
		case semantics.ValReal:
			return &JValue{RealValue: f64(val.Const.Real)}
		case semantics.ValBool:
			return &JValue{BoolValue: boolean(val.Const.Bool)}
		case semantics.ValInfinity:
			return &JValue{Infinity: boolean(true)}
		default:
			return &JValue{Null: text("unsupported const kind")}
		}
	case runtime.ValString:
		return &JValue{StringValue: text(val.Str())}
	case runtime.ValNull:
		return &JValue{Null: text("")}
	case runtime.ValInstance:
		return &JValue{InstanceId: i64(val.Instance)}
	case runtime.ValSequence:
		var elements []*JValue
		if val.Sequence() != nil {
			for _, elem := range val.Sequence().Elements() {
				elements = append(elements, valueToProto(rt, elem, idx))
			}
		}
		return &JValue{Sequence: &JValueSequence{Elements: elements}}
	case runtime.ValSet:
		return setToProto(rt, val, idx)
	case runtime.ValVariant:
		// The wire Value has no variant form: the object a selected variant
		// materialized is reported by identity, a valueless selection as unsupported.
		if id, ok := val.Object(); ok {
			return &JValue{InstanceId: i64(id)}
		}
		return &JValue{Null: text("unsupported: variant selection")}
	case runtime.ValQuantity:
		pq := quantityToProto(val.Quantity())
		if pq == nil {
			return &JValue{Null: text("unsupported: quantity with a non-numeric magnitude")}
		}
		return &JValue{Quantity: pq}
	case runtime.ValEnumLiteral:
		return enumLiteralValueToProto(val, idx)
	case runtime.ValComplex:
		return &JValue{Complex: complexToProto(val.Complex())}
	case runtime.ValArray:
		return &JValue{Array: arrayToProto(rt, val.Array(), idx)}
	case runtime.ValVector:
		return &JValue{Vector: vectorToProto(val.Vector())}
	case runtime.ValVectorQuantity:
		pvq := vectorQuantityToProto(val.VectorQuantity())
		if pvq == nil {
			return &JValue{Null: text("unsupported: vector quantity with a non-numeric component")}
		}
		return &JValue{VectorQuantity: pvq}
	case runtime.ValMeasurementRef:
		return &JValue{MeasurementRef: measurementRefToProto(val.MeasurementRef())}
	case runtime.ValFunction:
		if val.FunctionClosesOverBody() {
			return &JValue{Null: text("unsupported: function " + val.FunctionName() + " closing over a body's bindings")}
		}
		return &JValue{Function: functionToProto(val, idx)}
	case runtime.ValTensorQuantity:
		ptq := tensorQuantityToProto(val.TensorQuantity())
		if ptq == nil {
			return &JValue{Null: text("unsupported: tensor quantity with a non-numeric component")}
		}
		return &JValue{TensorQuantity: ptq}
	case runtime.ValMetaobject:
		meta := metaobjectToProto(val, idx)
		if meta == nil {
			return &JValue{Null: text("unsupported: metaobject of an element with no qualified name")}
		}
		return &JValue{Metaobject: meta}
	case runtime.ValUndetermined:
		return &JValue{Undetermined: undeterminedToProto(val.Undetermined())}
	case runtime.ValCoordinateFrame, runtime.ValCoordinateTransformation:
		// No wire arm carries a frame's axes or a transformation's placement.
		return &JValue{Null: text(unsupportedShown(val))}
	default:
		return &JValue{Null: text("unsupported")}
	}
}

// undeterminedToProto carries why a model-level result is open and how many
// values it would hold, the count's bounds as MultiplicityInfo spells them.
func undeterminedToProto(u *runtime.Undetermined) *JUndetermined {
	count := u.Count()
	return &JUndetermined{
		Reason: u.Reason(),
		Count:  &JMultiplicityInfo{Lower: boundText(count.Lower), Upper: boundText(count.Upper)},
	}
}

// functionToProto names a function by the calc declaration it is a value of and
// the object it closes over, if any.
func functionToProto(val runtime.Value, idx *symbols.Index) *JFunction {
	fn := &JFunction{CalcId: val.FunctionName()}
	if idx != nil && val.Function() != nil {
		fn.CalcId = idx.GetFQN(val.Function())
	}
	if self := val.FunctionSelf(); self != nil {
		fn.SelfId = I64(self.ID)
	}
	return fn
}

// enumLiteralValueToProto sends a value that is an enumeration literal — its
// identity alone, or a scalar the literal equals — through the enum_literal arm.
func enumLiteralValueToProto(val runtime.Value, idx *symbols.Index) *JValue {
	lit := enumLiteralToProto(val, idx)
	if lit == nil {
		return &JValue{Null: text("unsupported: unresolved enumeration literal")}
	}
	return &JValue{EnumLiteral: lit}
}

// metaobjectToProto names a metaobject by the element it reflects, which is its
// identity, and by that element's own metaclass. Nil when either is unnamed.
func metaobjectToProto(val runtime.Value, idx *symbols.Index) *JMetaobject {
	element, metaclass := val.MetaobjectElement(), val.MetaobjectClass()
	if element == nil || metaclass == nil || element.Name == "" || metaclass.Name == "" {
		return nil
	}
	elementID, metaclassID := element.Name, metaclass.Name
	if idx != nil {
		elementID, metaclassID = idx.GetFQN(element), idx.GetFQN(metaclass)
	}
	return &JMetaobject{ElementId: elementID, MetaclassId: metaclassID}
}

// enumLiteralToProto names a literal by the declaration it is, which is its
// identity, and by the enumeration declaring it, carrying the scalar a valued
// literal equals. Nil for an unresolved literal.
func enumLiteralToProto(val runtime.Value, idx *symbols.Index) *JEnumLiteral {
	sym := val.EnumerationLiteral()
	if sym == nil {
		return nil
	}
	lit := &JEnumLiteral{
		LiteralId: idx.GetFQN(sym),
		Name:      runtime.NewEnumLiteral(sym).LiteralText(),
	}
	if enum := semantics.EnumerationOwning(sym); enum != nil {
		lit.EnumerationId = idx.GetFQN(enum)
	}
	if val.Kind != runtime.ValEnumLiteral {
		lit.Value = valueToProto(nil, val.Scalar(), idx)
	}
	return lit
}

// complexToProto marshals a complex number as its rectangular parts.
func complexToProto(z complex128) *JComplex {
	return &JComplex{Real: F64(real(z)), Imaginary: F64(imag(z))}
}

// protoToComplex is the complex number a Complex message carries.
func protoToComplex(pc *JComplex) complex128 {
	return complex(float64(pc.Real), float64(pc.Imaginary))
}

// arrayToProto marshals an array as its dimensions and its row-major elements,
// each converted as any value is.
func arrayToProto(rt *runtime.Context, a *runtime.Array, idx *symbols.Index) *JArray {
	dimensions := make([]I64, len(a.Dimensions))
	for i, d := range a.Dimensions {
		dimensions[i] = I64(d)
	}
	pa := &JArray{Dimensions: dimensions}
	for _, elem := range a.Elements {
		pa.Elements = append(pa.Elements, valueToProto(rt, elem, idx))
	}
	return pa
}

// setToProto marshals a set's distinct elements in canonical order, each
// converted as any value is. A member with no wire form withholds the whole
// set: sent as nulls, two such members would read as one repeated.
func setToProto(rt *runtime.Context, val runtime.Value, idx *symbols.Index) *JValue {
	ps := &JValueSet{}
	if s := val.Set(); s != nil {
		for _, elem := range s.Elements() {
			pv := valueToProto(rt, elem, idx)
			if reason, ok := unsupportedReason(pv); ok {
				return &JValue{Null: text(unsupportedSet(val, reason))}
			}
			ps.Elements = append(ps.Elements, pv)
		}
	}
	return &JValue{Set: ps}
}

// unsupportedNullPrefix opens the non-empty null arm a value without a wire
// form crosses as; the reason follows it.
const unsupportedNullPrefix = "unsupported: "

// unsupportedShown is the null arm a value of a kind the wire has no arm for
// crosses as, naming the kind and the value as the REPL shows it.
func unsupportedShown(shown runtime.Value) string {
	return unsupportedNullPrefix + shown.Kind.String() + " " + runtime.FormatValue(shown)
}

// unsupportedReason reads the non-empty null arm a value without a wire form
// crosses as; the SysML `null` is the empty one.
func unsupportedReason(pv *JValue) (string, bool) {
	if pv.Null != nil && *pv.Null != "" {
		return strings.TrimPrefix(*pv.Null, unsupportedNullPrefix), true
	}
	return "", false
}

// unsupportedSet is the null arm a set holding a member without a wire form
// crosses as, naming the set and the member's reason.
func unsupportedSet(shown runtime.Value, reason string) string {
	return unsupportedNullPrefix + runtime.ValSet.String() + " " + runtime.FormatValue(shown) + " holding " + reason
}

// tensorQuantityToProto marshals a tensor as its dimensions and one Quantity
// per row-major component; nil if a component's magnitude is not a number.
func tensorQuantityToProto(tq *runtime.TensorQuantity) *JTensorQuantity {
	if tq == nil {
		return nil
	}
	dimensions := make([]I64, len(tq.Dimensions))
	for i, d := range tq.Dimensions {
		dimensions[i] = I64(d)
	}
	ptq := &JTensorQuantity{Dimensions: dimensions}
	for i := range tq.Num {
		pq := quantityToProto(&runtime.Quantity{Num: tq.Num[i], Unit: tq.Units[i]})
		if pq == nil {
			return nil
		}
		ptq.Components = append(ptq.Components, pq)
	}
	return ptq
}

// vectorToProto marshals a vector's components, Integer and Real kept apart.
func vectorToProto(v *runtime.Vector) *JVector {
	pv := &JVector{}
	for _, elem := range v.Elements {
		pv.Components = append(pv.Components, valueToProto(nil, runtime.Value{Kind: runtime.ValConst, Const: elem}, nil))
	}
	return pv
}

// vectorQuantityToProto marshals a vector quantity as one Quantity per axis;
// nil if a component's magnitude is not a number.
func vectorQuantityToProto(vq *runtime.VectorQuantity) *JVectorQuantity {
	pvq := &JVectorQuantity{}
	for i := range vq.Num {
		pq := quantityToProto(&runtime.Quantity{Num: vq.Num[i], Unit: vq.Units[i]})
		if pq == nil {
			return nil
		}
		pvq.Components = append(pvq.Components, pq)
	}
	return pvq
}

// measurementRefToProto marshals a bare reference: the unit as written, what it
// reduces to, and the one declaration it names when it names one.
func measurementRefToProto(ref *runtime.MeasurementRef) *JMeasurementRef {
	pm := &JMeasurementRef{Unit: ref.Unit.Text, UnitTerm: unitTermToProto(ref.Unit.Term)}
	if decl := ref.Declaration(); decl != nil {
		pm.UnitId = symbols.FQNOf(decl)
	}
	return pm
}

// quantityToProto marshals a quantity: the magnitude in the unit written, plus
// what that unit reduces to. Reports nil for a magnitude that is not a number.
func quantityToProto(q *runtime.Quantity) *JQuantity {
	if q == nil {
		return nil
	}
	pq := &JQuantity{Unit: q.Unit.Text, UnitTerm: unitTermToProto(q.Unit.Term)}
	switch q.Num.Kind {
	case semantics.ValInt:
		pq.IntMagnitude = i64(q.Num.Int)
	case semantics.ValReal:
		pq.RealMagnitude = f64(q.Num.Real)
	default:
		return nil
	}
	return pq
}

// unitTermToProto marshals a unit's reduction to base units, naming each base
// unit by qualified name: a symbol pointer means nothing to another process.
func unitTermToProto(term semantics.UnitTerm) *JUnitTerm {
	pt := &JUnitTerm{ScaleNum: F64(term.Scale.Num), ScaleDen: F64(term.Scale.Den)}
	for _, f := range term.Factors {
		pt.Factors = append(pt.Factors, &JUnitFactor{
			UnitId:   symbols.FQNOf(f.Unit),
			Exponent: F64(f.Exponent),
		})
	}
	return pt
}

var (
	// The conversion errors below are protoconv's, carried over verbatim so a
	// wire form the service refuses is refused under the same message.

	errQuantityNeedsIndex = errors.New("a quantity needs the model's symbols to be converted from the wire")

	errUnknownBaseUnit = errors.New("unknown base unit")

	errNotAMeasurementUnit = errors.New("not a measurement unit")

	errScaleNotAFactor = errors.New("measurement scale is not a unit factor")

	errUnitScaleUnusable = errors.New("unit scale is not a usable ratio")

	errUnitExponentUnusable = errors.New("unit exponent is not a finite number")

	errUnitNotReduced = errors.New("unit carries no reduction to base units")

	errUnitTextMismatch = errors.New("unit as written does not reduce to its unit_term")

	errUnsetNotAccepted = errors.New("unset is not a value a caller can supply")

	errUndeterminedNotAccepted = errors.New("undetermined is not a value a caller can supply")

	errInfinityNotAsserted = errors.New("the infinity arm states no value unless it is true")

	errArrayDimensionNotPositive = errors.New("array dimension is not positive")

	errArrayShapeMismatch = errors.New("array elements do not fill its dimensions")

	errVectorComponentNotNumeric = errors.New("vector component is not a number")

	errVectorQuantityEmpty = errors.New("vector quantity has no components")

	errMeasurementRefNeedsIndex = errors.New("a measurement reference needs the model's symbols to be converted from the wire")

	errMeasurementRefEmpty = errors.New("measurement reference names no unit")

	errUnknownMeasurementUnit = errors.New("unknown measurement unit")

	errUnitIDMismatch = errors.New("unit as written does not spell unit_id")

	errFunctionNeedsRuntime = errors.New("function needs a runtime to bind its calc")

	errFunctionUnbound = errors.New("function names no calc of this model")

	errMetaobjectUnbound = errors.New("metaobject names no element of this model")

	errMetaobjectAmbiguous = errors.New("metaobject names more than one element of this model")

	errMetaclassMismatch = errors.New("metaclass_id is not the element's metaclass")

	errSetElementRepeated = errors.New("set element is repeated")

	errTensorDimensionNotPositive = errors.New("tensor dimension is not positive")

	errTensorShapeMismatch = errors.New("tensor components do not fill its dimensions")

	errTensorComponentMissing = errors.New("tensor component carries no quantity")
)

// protoToRuntimeValue converts a wire value to a runtime.Value in the model idx
// and sem describe, resolving a quantity's base units against them; rt is what
// a function's calc is bound in and may be nil for a value naming no function.
func protoToRuntimeValue(rt *runtime.Context, pv *JValue, idx *symbols.Index, sem *semantics.Model) (runtime.Value, error) {
	if pv == nil {
		return runtime.Value{Kind: runtime.ValNull}, nil
	}
	switch {
	case pv.Unset != nil:
		return runtime.Value{}, errUnsetNotAccepted
	case pv.Undetermined != nil:
		return runtime.Value{}, errUndeterminedNotAccepted
	case pv.Quantity != nil:
		return protoToQuantity(pv.Quantity, idx, sem)
	case pv.EnumLiteral != nil:
		return enumLiteralFromProto(rt, pv.EnumLiteral, idx, sem)
	case pv.Function != nil:
		return functionFromProto(rt, pv.Function, idx)
	case pv.Metaobject != nil:
		return metaobjectFromProto(pv.Metaobject, idx, sem)
	case pv.Sequence != nil:
		seq := runtime.NewSequence()
		for _, elem := range pv.Sequence.Elements {
			val, err := protoToRuntimeValue(rt, elem, idx, sem)
			if err != nil {
				return runtime.Value{}, err
			}
			seq.Append(val)
		}
		return runtime.NewSequenceValue(seq), nil
	case pv.Set != nil:
		return protoToSet(rt, pv.Set, idx, sem)
	case pv.TensorQuantity != nil:
		return protoToTensorQuantity(pv.TensorQuantity, idx, sem)
	case pv.Array != nil:
		return protoToArray(rt, pv.Array, idx, sem)
	case pv.Vector != nil:
		return protoToVector(pv.Vector)
	case pv.VectorQuantity != nil:
		return protoToVectorQuantity(pv.VectorQuantity, idx, sem)
	case pv.MeasurementRef != nil:
		return protoToMeasurementRef(pv.MeasurementRef, idx, sem)
	case pv.Infinity != nil:
		if !*pv.Infinity {
			return runtime.Value{}, errInfinityNotAsserted
		}
		return protoToScalar(pv), nil
	default:
		return protoToScalar(pv), nil
	}
}

// functionFromProto binds a function to the calc its calc_id names in rt's
// model, read as a value in its own scope. Objects live only within the call that
// created them, so a self_id names none of this call's: it is refused rather
// than matched to whichever object this call happened to number the same.
func functionFromProto(rt *runtime.Context, fn *JFunction, idx *symbols.Index) (runtime.Value, error) {
	if fn == nil || fn.CalcId == "" {
		return runtime.Value{}, fmt.Errorf("%w: calc_id is empty", errFunctionUnbound)
	}
	if rt == nil || idx == nil {
		return runtime.Value{}, fmt.Errorf("%w: function %s", errFunctionNeedsRuntime, fn.CalcId)
	}
	if fn.SelfId != 0 {
		return runtime.Value{}, fmt.Errorf(
			"%w: %s: self_id %d names no object of this call: an object lives only within the response that created it",
			errFunctionUnbound, fn.CalcId, int64(fn.SelfId))
	}
	for _, sym := range idx.LookupQualified(fn.CalcId) {
		val, isFunction, err := rt.FunctionValue(sym)
		if !isFunction {
			continue
		}
		if err != nil {
			return runtime.Value{}, fmt.Errorf("%w: %s: %v", errFunctionUnbound, fn.CalcId, err)
		}
		return val, nil
	}
	return runtime.Value{}, fmt.Errorf("%w: %s is not a calc", errFunctionUnbound, fn.CalcId)
}

// metaobjectFromProto binds a metaobject to the one element its element_id
// names, reflected on as the metaclass the model classifies it by. A name two
// declarations share identifies neither, and a metaclass_id naming another
// metaclass is refused rather than read as a cast.
func metaobjectFromProto(meta *JMetaobject, idx *symbols.Index, sem *semantics.Model) (runtime.Value, error) {
	if meta == nil || meta.ElementId == "" {
		return runtime.Value{}, fmt.Errorf("%w: element_id is empty", errMetaobjectUnbound)
	}
	if idx == nil || sem == nil {
		return runtime.Value{}, fmt.Errorf("%w: metaobject %s: no model to resolve it against", errMetaobjectUnbound, meta.ElementId)
	}
	var element, metaclass *symbols.Symbol
	for _, sym := range idx.LookupQualified(meta.ElementId) {
		mc := sem.MetaclassOf(sym)
		if mc == nil || sym == element {
			continue
		}
		if element != nil {
			return runtime.Value{}, fmt.Errorf("%w: %s", errMetaobjectAmbiguous, meta.ElementId)
		}
		element, metaclass = sym, mc
	}
	if element == nil {
		return runtime.Value{}, fmt.Errorf("%w: %s", errMetaobjectUnbound, meta.ElementId)
	}
	if meta.MetaclassId != "" && meta.MetaclassId != idx.GetFQN(metaclass) {
		return runtime.Value{}, fmt.Errorf("%w: %s is classified by %s, not %s",
			errMetaclassMismatch, meta.ElementId, idx.GetFQN(metaclass), meta.MetaclassId)
	}
	return runtime.NewMetaobject(element, metaclass), nil
}

// protoToSet rebuilds a set from elements sent in any order, refusing one sent
// twice rather than reading the two as one.
func protoToSet(rt *runtime.Context, ps *JValueSet, idx *symbols.Index, sem *semantics.Model) (runtime.Value, error) {
	set := runtime.NewSetIn(rt)
	for i, elem := range ps.Elements {
		val, err := protoToRuntimeValue(rt, elem, idx, sem)
		if err != nil {
			return runtime.Value{}, err
		}
		if set.Contains(val) {
			return runtime.Value{}, fmt.Errorf("%w: element %d, %s", errSetElementRepeated, i+1, runtime.FormatValue(val))
		}
		set.Add(val)
	}
	return runtime.NewSetValue(set), nil
}

// protoToTensorQuantity rebuilds a tensor of any rank, refusing a shape its
// components do not fill, each component read exactly as a scalar Quantity is.
func protoToTensorQuantity(ptq *JTensorQuantity, idx *symbols.Index, sem *semantics.Model) (runtime.Value, error) {
	dimensions := make([]int64, len(ptq.Dimensions))
	for i, d := range ptq.Dimensions {
		dimensions[i] = int64(d)
	}
	if err := checkTensorShape(dimensions, len(ptq.Components)); err != nil {
		return runtime.Value{}, err
	}
	num := make([]semantics.Value, 0, len(ptq.Components))
	units := make([]runtime.Unit, 0, len(ptq.Components))
	for i, comp := range ptq.Components {
		if comp == nil {
			return runtime.Value{}, fmt.Errorf("%w: component %d", errTensorComponentMissing, i+1)
		}
		val, err := protoToQuantity(comp, idx, sem)
		if err != nil {
			return runtime.Value{}, fmt.Errorf("component %d: %w", i+1, err)
		}
		q := val.Quantity()
		num = append(num, q.Num)
		units = append(units, q.Unit)
	}
	return runtime.NewTensorQuantityValue(dimensions, num, units), nil
}

// protoToArray rebuilds an array, refusing a shape its elements do not fill
// rather than reading them under some other shape.
func protoToArray(rt *runtime.Context, pa *JArray, idx *symbols.Index, sem *semantics.Model) (runtime.Value, error) {
	dimensions := make([]int64, len(pa.Dimensions))
	for i, d := range pa.Dimensions {
		dimensions[i] = int64(d)
	}
	if err := checkArrayShape(dimensions, len(pa.Elements)); err != nil {
		return runtime.Value{}, err
	}
	elements := make([]runtime.Value, 0, len(pa.Elements))
	for _, elem := range pa.Elements {
		val, err := protoToRuntimeValue(rt, elem, idx, sem)
		if err != nil {
			return runtime.Value{}, err
		}
		elements = append(elements, val)
	}
	return runtime.NewArrayValue(dimensions, elements), nil
}

// checkArrayShape reports whether count elements fill dimensions in row-major
// order: every dimension positive and their product (one for rank 0) count.
func checkArrayShape(dimensions []int64, count int) error {
	return checkShape(dimensions, count, errArrayDimensionNotPositive, errArrayShapeMismatch)
}

// checkTensorShape is checkArrayShape for a tensor's components, reported as
// the tensor errors.
func checkTensorShape(dimensions []int64, count int) error {
	return checkShape(dimensions, count, errTensorDimensionNotPositive, errTensorShapeMismatch)
}

func checkShape(dimensions []int64, count int, notPositive, mismatch error) error {
	size := int64(1)
	for i, d := range dimensions {
		if d < 1 {
			return fmt.Errorf("%w: dimension %d is %d", notPositive, i+1, d)
		}
		if size > math.MaxInt64/d {
			return fmt.Errorf("%w: flattenedSize of dimensions %v exceeds the Integer range", mismatch, dimensions)
		}
		size *= d
	}
	if size != int64(count) {
		return fmt.Errorf("%w: %d elements under dimensions %v (flattenedSize %d)",
			mismatch, count, dimensions, size)
	}
	return nil
}

// protoToVector rebuilds a vector from components that are each an Integer or
// a Real, refusing any other arm rather than reading it as a number.
func protoToVector(pv *JVector) (runtime.Value, error) {
	components := make([]semantics.Value, 0, len(pv.Components))
	for i, comp := range pv.Components {
		num, err := protoToNumber(comp)
		if err != nil {
			return runtime.Value{}, fmt.Errorf("%w: component %d", err, i+1)
		}
		components = append(components, num)
	}
	return runtime.NewVectorValue(components), nil
}

// protoToNumber is the Integer or Real a value holds; any other arm is an error.
func protoToNumber(pv *JValue) (semantics.Value, error) {
	switch {
	case pv.IntValue != nil:
		return semantics.Value{Kind: semantics.ValInt, Int: int64(*pv.IntValue)}, nil
	case pv.RealValue != nil:
		return semantics.Value{Kind: semantics.ValReal, Real: float64(*pv.RealValue)}, nil
	}
	return semantics.Value{}, errVectorComponentNotNumeric
}

// protoToVectorQuantity rebuilds a vector quantity axis by axis, each
// component read exactly as a scalar Quantity is.
func protoToVectorQuantity(pvq *JVectorQuantity, idx *symbols.Index, sem *semantics.Model) (runtime.Value, error) {
	if len(pvq.Components) == 0 {
		return runtime.Value{}, errVectorQuantityEmpty
	}
	num := make([]semantics.Value, 0, len(pvq.Components))
	units := make([]runtime.Unit, 0, len(pvq.Components))
	for i, comp := range pvq.Components {
		if comp == nil {
			return runtime.Value{}, fmt.Errorf("%w: component %d carries no quantity", errVectorComponentNotNumeric, i+1)
		}
		val, err := protoToQuantity(comp, idx, sem)
		if err != nil {
			return runtime.Value{}, fmt.Errorf("component %d: %w", i+1, err)
		}
		q := val.Quantity()
		num = append(num, q.Num)
		units = append(units, q.Unit)
	}
	return runtime.NewVectorQuantityValue(num, units), nil
}

// protoToQuantity rebuilds a quantity from the wire: the magnitude as sent, in
// the unit as written — read as the product of the named units it composes, so
// an operation over it cancels and merges them — over the base units idx
// resolves its reduction to.
func protoToQuantity(pq *JQuantity, idx *symbols.Index, sem *semantics.Model) (runtime.Value, error) {
	if pq == nil {
		return runtime.Value{Kind: runtime.ValNull}, nil
	}
	if (idx == nil || sem == nil) && len(pq.UnitTerm.Factors) > 0 {
		return runtime.Value{}, fmt.Errorf("%w: %s", errQuantityNeedsIndex, pq.Unit)
	}
	if pq.UnitTerm == nil && pq.Unit != "" {
		return runtime.Value{}, fmt.Errorf("%w: %s", errUnitNotReduced, pq.Unit)
	}
	term, err := protoToUnitTerm(pq.UnitTerm, idx, sem)
	if err != nil {
		return runtime.Value{}, err
	}

	var num semantics.Value
	switch {
	case pq.IntMagnitude != nil:
		num = semantics.Value{Kind: semantics.ValInt, Int: int64(*pq.IntMagnitude)}
	case pq.RealMagnitude != nil:
		num = semantics.Value{Kind: semantics.ValReal, Real: float64(*pq.RealMagnitude)}
	default:
		return runtime.Value{}, fmt.Errorf("quantity in %q carries no magnitude", pq.Unit)
	}

	product, term, err := unitProductOfText(pq.Unit, term, idx, sem)
	if err != nil {
		return runtime.Value{}, err
	}
	text := pq.Unit
	if text == "" && !product.IsEmpty() {
		text = product.String()
	}
	unit := runtime.Unit{Text: text, Product: product, Term: term}
	return runtime.NewQuantityValue(&runtime.Quantity{Num: num, Unit: unit}), nil
}

// protoToMeasurementRef rebuilds a bare reference from the wire: by the
// declaration unit_id names, checked against the unit text and reduction sent
// with it, or else by the unit text read exactly as a Quantity's is.
func protoToMeasurementRef(pm *JMeasurementRef, idx *symbols.Index, sem *semantics.Model) (runtime.Value, error) {
	if pm.Unit == "" && pm.UnitTerm == nil && pm.UnitId == "" {
		return runtime.Value{}, errMeasurementRefEmpty
	}
	if (idx == nil || sem == nil) && (len(pm.UnitTerm.Factors) > 0 || pm.UnitId != "") {
		return runtime.Value{}, fmt.Errorf("%w: %s", errMeasurementRefNeedsIndex, pm.Unit)
	}
	if pm.UnitTerm == nil {
		return runtime.Value{}, fmt.Errorf("%w: %s", errUnitNotReduced, cmp.Or(pm.Unit, pm.UnitId))
	}
	term, err := protoToUnitTerm(pm.UnitTerm, idx, sem)
	if err != nil {
		return runtime.Value{}, err
	}
	if pm.UnitId != "" {
		return declaredMeasurementRef(pm.UnitId, pm.Unit, term, idx, sem)
	}
	product, term, err := unitProductOfText(pm.Unit, term, idx, sem)
	if err != nil {
		return runtime.Value{}, err
	}
	text := pm.Unit
	if text == "" && !product.IsEmpty() {
		text = product.String()
	}
	return runtime.NewMeasurementRefValue(runtime.Unit{Text: text, Product: product, Term: term}), nil
}

// declaredMeasurementRef is the reference to the unit declaration id names, refused
// unless it is a measurement unit reducing to term that text, if any, spells.
func declaredMeasurementRef(id, text string, term semantics.UnitTerm, idx *symbols.Index, sem *semantics.Model) (runtime.Value, error) {
	matches := idx.LookupQualified(id)
	if len(matches) != 1 {
		return runtime.Value{}, fmt.Errorf("%w: %s", errUnknownMeasurementUnit, id)
	}
	unit, ok := sem.MeasurementUnitOf(matches[0])
	if !ok {
		return runtime.Value{}, fmt.Errorf("%w: %s", errNotAMeasurementUnit, id)
	}
	declared, err := sem.UnitTermOf(unit)
	if err != nil {
		return runtime.Value{}, fmt.Errorf("%w: %s: %w", errUnitNotReduced, id, err)
	}
	if !reducesTo(declared, term) {
		return runtime.Value{}, fmt.Errorf("%w: %s reduces to %s, unit_term is %s", errUnitTextMismatch, id, declared, term)
	}
	if text != "" && !textSpellsUnit(text, unit, idx, sem) {
		return runtime.Value{}, fmt.Errorf("%w: %s is not %s", errUnitIDMismatch, text, id)
	}
	return runtime.DeclaredMeasurementRef(sem, unit, text, declared), nil
}

// textSpellsUnit reports whether unit text is one name for the declaration unit:
// its symbol or name (`km`, `kilometre`), or a qualified name resolving to it.
func textSpellsUnit(text string, unit *symbols.Symbol, idx *symbols.Index, sem *semantics.Model) bool {
	p := parser.New(source.New("<unit>", []byte(text)))
	expr := p.ParseExpression()
	if expr == nil || len(p.Diagnostics) > 0 || p.Offset() != len(text) {
		return false
	}
	spellings := []string{source.NameText(unit.Name)}
	if unit.ShortName != "" {
		spellings = append(spellings, source.NameText(unit.ShortName))
	}
	product, err := sem.UnitProductOfExprBy(expr, func(qn *ast.QualifiedName) (*symbols.Symbol, bool) {
		if len(qn.Parts) == 1 {
			return unit, slices.Contains(spellings, source.NameText(qn.Parts[0].Text))
		}
		matches := idx.LookupQualified(qn.Text())
		if len(matches) != 1 {
			return nil, false
		}
		sym, ok := sem.MeasurementUnitOf(matches[0])
		return unit, ok && sym == unit
	})
	if err != nil || len(product.Powers) != 1 {
		return false
	}
	return product.Powers[0].Unit == unit && product.Powers[0].Exponent == 1
}

// unitProductOfText reads unit text as a product of the model's units that reduces
// to term; a short name is the one unit so named that fits. Text that does not read
// so keeps the factors it can name, the rest opaque (see partialUnitProduct); text
// that is no unit expression is one opaque unit. The reduction returned is the
// model's where the text is read in full, else term as sent.
func unitProductOfText(text string, term semantics.UnitTerm, idx *symbols.Index, sem *semantics.Model) (semantics.UnitProduct, semantics.UnitTerm, error) {
	if text == "" {
		return unnamedUnitProduct(term), term, nil
	}
	opaque := semantics.OpaqueUnitProduct(text, term)
	if idx == nil || sem == nil {
		return opaque, term, nil
	}
	p := parser.New(source.New("<unit>", []byte(text)))
	expr := p.ParseExpression()
	if expr == nil || len(p.Diagnostics) > 0 || p.Offset() != len(text) {
		return opaque, term, nil
	}
	unitAt := func(fqn string) (*symbols.Symbol, bool) {
		matches := idx.LookupQualified(fqn)
		if len(matches) != 1 {
			return nil, false
		}
		return measurementRefOf(matches[0], sem)
	}
	var short []*ast.QualifiedName
	product, err := sem.UnitProductOfExprBy(expr, func(qn *ast.QualifiedName) (*symbols.Symbol, bool) {
		if sym, ok := unitAt(qn.Text()); ok {
			return sym, true
		}
		if len(qn.Parts) == 1 && !slices.Contains(short, qn) {
			short = append(short, qn)
		}
		return nil, false
	})
	if err != nil {
		return opaque, term, nil
	}
	if len(short) == 0 {
		implied, ok := impliedTerm(product, sem)
		if !ok {
			return partialUnitProduct(expr, opaque, term, unitAt, idx, sem), term, nil
		}
		if !reducesTo(implied, term) {
			return semantics.UnitProduct{}, semantics.UnitTerm{}, fmt.Errorf("%w: %s reduces to %s, unit_term is %s",
				errUnitTextMismatch, text, implied, term)
		}
		return product, implied, nil
	}

	readings := shortUnitReadings(short, idx, sem)
	var matches []shortUnitReading
	for _, reading := range readings {
		product, err := sem.UnitProductOfExprBy(expr, func(qn *ast.QualifiedName) (*symbols.Symbol, bool) {
			if sym, ok := unitAt(qn.Text()); ok {
				return sym, true
			}
			sym, ok := reading.units[qn]
			return sym, ok
		})
		if err != nil {
			continue
		}
		implied, ok := impliedTerm(product, sem)
		if !ok || !reducesTo(implied, term) {
			continue
		}
		// Two readings of one product, `m*m` as A::m·B::m and as B::m·A::m, are one reading.
		if slices.ContainsFunc(matches, func(m shortUnitReading) bool { return m.product.Equal(product) }) {
			continue
		}
		reading.product, reading.term = product, implied
		matches = append(matches, reading)
	}
	if len(matches) > 1 {
		matches = slices.DeleteFunc(matches, func(r shortUnitReading) bool {
			return !r.besideBaseUnits(term)
		})
	}
	if len(matches) != 1 {
		return partialUnitProduct(expr, opaque, term, unitAt, idx, sem), term, nil
	}
	return matches[0].product, matches[0].term, nil
}

// partialUnitProduct reads a unit text name by name — a qualified name or a short name
// one unit bears is that unit, any other an opaque factor — so the units read still cancel.
// Text whose every name reads, yet contradicts term, is opaque as a whole.
func partialUnitProduct(
	expr ast.Node,
	opaque semantics.UnitProduct,
	term semantics.UnitTerm,
	unitAt func(string) (*symbols.Symbol, bool),
	idx *symbols.Index,
	sem *semantics.Model,
) semantics.UnitProduct {
	unreadNames := map[string]int{}
	product, err := sem.UnitProductOfExprBy(expr, func(qn *ast.QualifiedName) (*symbols.Symbol, bool) {
		if sym, ok := unitAt(qn.Text()); ok {
			return sym, true
		}
		if len(qn.Parts) == 1 {
			if units := unitsNamed(qn.Text(), idx, sem); len(units) == 1 {
				return units[0], true
			}
		}
		unreadNames[qn.Text()]++
		return nil, false
	})
	if err != nil {
		return opaque
	}
	// One unread name written twice is two units the text cannot tell apart.
	for _, n := range unreadNames {
		if n > 1 {
			return opaque
		}
	}
	known := semantics.UnitTerm{Scale: semantics.UnitScale(1)}
	var unread []int
	for i, f := range product.Powers {
		if f.Unit == nil {
			unread = append(unread, i)
			continue
		}
		// A scale composed with anything is text no reduction is; nothing of it is read.
		if sem.IsMeasurementScale(f.Unit) {
			return opaque
		}
		factor, err := sem.UnitTermOf(f.Unit)
		if err != nil {
			return opaque
		}
		known = known.Times(factor.Pow(f.Exponent))
	}
	if len(unread) == 0 {
		return opaque
	}
	// A lone opaque factor is what term leaves once the units read are taken out.
	if len(unread) == 1 {
		f := &product.Powers[unread[0]]
		reduces := term.DividedBy(known).Pow(1 / f.Exponent)
		f.DimensionOne, f.Reduces = reduces.Dimensionless(), &reduces
	}
	return product
}

// shortUnitReading is one assignment of a unit text's short names, occurrence by
// occurrence, to units: `m*m` may name two units both written m.
type shortUnitReading struct {
	units   map[*ast.QualifiedName]*symbols.Symbol
	product semantics.UnitProduct
	term    semantics.UnitTerm
}

// besideBaseUnits reports whether every unit read is declared beside a base unit of term.
func (r shortUnitReading) besideBaseUnits(term semantics.UnitTerm) bool {
	namespaces := baseUnitNamespaces(term)
	for _, sym := range r.units {
		if !slices.Contains(namespaces, namespaceOf(sym)) {
			return false
		}
	}
	return true
}

// maxShortUnitReadings bounds the readings tried for one unit text.
const maxShortUnitReadings = 1024

// shortUnitReadings enumerates, in a fixed order, every assignment of the short
// name occurrences to units so named; none if a name has no unit or there are too many.
func shortUnitReadings(names []*ast.QualifiedName, idx *symbols.Index, sem *semantics.Model) []shortUnitReading {
	candidates := make([][]*symbols.Symbol, len(names))
	total := 1
	for i, qn := range names {
		candidates[i] = unitsNamed(qn.Text(), idx, sem)
		total *= len(candidates[i])
		if total == 0 || total > maxShortUnitReadings {
			return nil
		}
	}
	readings := make([]shortUnitReading, 0, total)
	for k := range total {
		units := make(map[*ast.QualifiedName]*symbols.Symbol, len(names))
		rem := k
		for i, qn := range names {
			units[qn] = candidates[i][rem%len(candidates[i])]
			rem /= len(candidates[i])
		}
		readings = append(readings, shortUnitReading{units: units})
	}
	return readings
}

// unitsNamed lists, once each in qualified-name order, the units and measurement
// scales under a short name.
func unitsNamed(name string, idx *symbols.Index, sem *semantics.Model) []*symbols.Symbol {
	var units []*symbols.Symbol
	for _, fqn := range idx.FQNsEndingIn(name, math.MaxInt) {
		for _, sym := range idx.LookupQualified(fqn) {
			if unit, ok := measurementRefOf(sym, sem); ok && !slices.Contains(units, unit) {
				units = append(units, unit)
			}
		}
	}
	return units
}

// measurementRefOf is the measurement unit or scale sym names; false for anything else.
func measurementRefOf(sym *symbols.Symbol, sem *semantics.Model) (*symbols.Symbol, bool) {
	if unit, ok := sem.MeasurementUnitOf(sym); ok {
		return unit, true
	}
	if sem.IsMeasurementScale(sym) {
		return sym, true
	}
	return nil, false
}

// termOfMeasurementRef reduces a unit to base units, and a measurement scale to
// itself: a point on it is commensurable with nothing but another point on it.
func termOfMeasurementRef(sym *symbols.Symbol, sem *semantics.Model) (semantics.UnitTerm, error) {
	if sem.IsMeasurementScale(sym) {
		return semantics.UnitTerm{Scale: semantics.UnitScale(1), Factors: []semantics.UnitFactor{{Unit: sym, Exponent: 1}}}, nil
	}
	return sem.UnitTermOf(sym)
}

// impliedTerm reduces a product of resolved units; false if one is unresolved or unreducible.
func impliedTerm(product semantics.UnitProduct, sem *semantics.Model) (semantics.UnitTerm, bool) {
	implied := semantics.UnitTerm{Scale: semantics.UnitScale(1)}
	for _, f := range product.Powers {
		if f.Unit == nil {
			return semantics.UnitTerm{}, false
		}
		factor, err := termOfMeasurementRef(f.Unit, sem)
		if err != nil {
			return semantics.UnitTerm{}, false
		}
		implied = implied.Times(factor.Pow(f.Exponent))
	}
	return implied, true
}

// reducesTo reports whether two reductions are one unit: commensurable at one scale.
func reducesTo(implied, term semantics.UnitTerm) bool {
	return implied.Commensurable(term) && sameScale(implied.Scale, term.Scale)
}

// namespaceOf is the qualified name of the namespace declaring sym, or "" for a root.
func namespaceOf(sym *symbols.Symbol) string {
	if sym.OwnerScope == nil || sym.OwnerScope.Owner() == nil {
		return ""
	}
	return symbols.FQNOf(sym.OwnerScope.Owner())
}

// baseUnitNamespaces lists, in factor order and once each, the namespaces
// declaring the base units a reduction is over.
func baseUnitNamespaces(term semantics.UnitTerm) []string {
	var out []string
	for _, f := range term.Factors {
		if f.Unit == nil {
			continue
		}
		if ns := namespaceOf(f.Unit); ns != "" && !slices.Contains(out, ns) {
			out = append(out, ns)
		}
	}
	return out
}

// unnamedUnitProduct is the unit of a quantity sent under no text: its base units
// at scale one, else the reduction as one opaque unit that names no dimension-one unit.
func unnamedUnitProduct(term semantics.UnitTerm) semantics.UnitProduct {
	if !sameScale(term.Scale, semantics.UnitScale(1)) {
		product := semantics.OpaqueUnitProduct(term.String(), term)
		product.Powers[0].DimensionOne = false
		return product
	}
	product := semantics.UnitProduct{}
	for _, f := range term.Factors {
		name := source.QualifiedNameText(symbols.FQNOf(f.Unit))
		product = product.Times(semantics.NamedUnitProduct(f.Unit, name, false).Pow(f.Exponent))
	}
	return product
}

// scaleTolerance is the relative difference two orders of composing one scale's
// factors can round to: a fraction of an ulp per multiplication, over at most dozens.
const scaleTolerance = 64 * 0x1p-52

// sameScale reports whether two scale ratios agree to within the rounding of
// composing them; a ratio further off is another scale, not noise.
func sameScale(a, b semantics.Scale) bool {
	return math.Abs(semantics.ConvertMagnitude(1, a, b)-1) <= scaleTolerance
}

// protoToUnitTerm rebuilds a unit's reduction, normalized so a term sent in any
// factor order is commensurable with the same unit derived in the model. A name
// the model does not declare uniquely as a measurement unit is an error, not a
// factor over whatever symbol it happened to resolve to.
func protoToUnitTerm(pt *JUnitTerm, idx *symbols.Index, sem *semantics.Model) (semantics.UnitTerm, error) {
	if pt == nil {
		// A magnitude sent under no unit at all: dimension one.
		return semantics.UnitTerm{Scale: semantics.UnitScale(1)}, nil
	}
	scale := semantics.Scale{Num: float64(pt.ScaleNum), Den: float64(pt.ScaleDen)}
	if scale.IsZero() || !finite(scale.Num) || !finite(scale.Den) {
		return semantics.UnitTerm{}, fmt.Errorf("%w: %g/%g", errUnitScaleUnusable, scale.Num, scale.Den)
	}
	term := semantics.UnitTerm{Scale: scale}
	var pointOn *symbols.Symbol
	for _, f := range pt.Factors {
		// An empty name is a lookup of the document root, so it is rejected here
		// rather than resolved to a symbol that measures nothing.
		if f.UnitId == "" {
			return semantics.UnitTerm{}, fmt.Errorf("%w: unit factor names no unit", errUnknownBaseUnit)
		}
		matches := idx.LookupQualified(f.UnitId)
		if len(matches) != 1 {
			return semantics.UnitTerm{}, fmt.Errorf("%w: %s", errUnknownBaseUnit, f.UnitId)
		}
		unit, ok := sem.MeasurementUnitOf(matches[0])
		if !ok {
			// A point on a measurement scale reduces to the scale alone.
			if !sem.IsMeasurementScale(matches[0]) {
				return semantics.UnitTerm{}, fmt.Errorf("%w: %s", errNotAMeasurementUnit, f.UnitId)
			}
			unit, pointOn = matches[0], matches[0]
		}
		term.Factors = append(term.Factors, semantics.UnitFactor{
			Unit:     unit,
			Exponent: float64(f.Exponent),
		})
	}
	// Checked after normalization: repeated factors sum their exponents, and
	// finite powers can overflow in that sum.
	term = term.Normalized()
	for _, f := range term.Factors {
		if !finite(f.Exponent) {
			return semantics.UnitTerm{}, fmt.Errorf("%w: %s**%g", errUnitExponentUnusable, symbols.FQNOf(f.Unit), f.Exponent)
		}
	}
	if pointOn != nil {
		if _, ok := sem.MeasurementScaleOf(term); !ok {
			return semantics.UnitTerm{}, fmt.Errorf("%w: %s in %s", errScaleNotAFactor, symbols.FQNOf(pointOn), term)
		}
	}
	return term, nil
}

// finite reports whether a wire double is a number a unit term can carry.
func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

// enumLiteralFromProto resolves a literal against the model, since a literal is
// the declaration it names: one the model does not declare has no identity here.
// The model says what a valued literal equals; the wire's value is consulted
// only when no runtime is at hand to evaluate the declaration.
func enumLiteralFromProto(rt *runtime.Context, lit *JEnumLiteral, idx *symbols.Index, sem *semantics.Model) (runtime.Value, error) {
	if lit == nil || lit.LiteralId == "" {
		return runtime.Value{}, fmt.Errorf("enumeration literal: literal_id names no declaration")
	}
	if idx == nil {
		return runtime.Value{}, fmt.Errorf("enumeration literal %s: no model to resolve it against", lit.LiteralId)
	}
	for _, sym := range idx.LookupQualified(lit.LiteralId) {
		if semantics.EnumerationOwning(sym) == nil {
			continue
		}
		if rt != nil {
			val, _, err := rt.EnumerationLiteralValue(sym)
			return val, err
		}
		if lit.Value == nil {
			return runtime.NewEnumLiteral(sym), nil
		}
		scalar, err := protoToRuntimeValue(nil, lit.Value, idx, sem)
		if err != nil {
			return runtime.Value{}, fmt.Errorf("enumeration literal %s: %w", lit.LiteralId, err)
		}
		return runtime.EnumeratedValue(sym, scalar), nil
	}
	return runtime.Value{}, fmt.Errorf("%s is not an enumeration literal of this model", lit.LiteralId)
}

// protoToScalar converts the arms of Value that name no symbol and hold no
// nested value.
func protoToScalar(pv *JValue) runtime.Value {
	switch {
	case pv.IntValue != nil:
		return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: int64(*pv.IntValue)}}
	case pv.RealValue != nil:
		return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: float64(*pv.RealValue)}}
	case pv.BoolValue != nil:
		return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: *pv.BoolValue}}
	case pv.StringValue != nil:
		return runtime.NewStringValue(*pv.StringValue)
	case pv.InstanceId != nil:
		return runtime.Value{Kind: runtime.ValInstance, Instance: int64(*pv.InstanceId)}
	case pv.Null != nil:
		return runtime.Value{Kind: runtime.ValNull}
	case pv.Complex != nil:
		return runtime.NewComplex(protoToComplex(pv.Complex))
	case pv.Infinity != nil:
		if !*pv.Infinity {
			return runtime.Value{Kind: runtime.ValNull}
		}
		return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInfinity}}
	default:
		return runtime.Value{Kind: runtime.ValNull}
	}
}

const (
	// maxGraphDepth bounds how deep Instantiate expands nested objects.
	maxGraphDepth = 8
	// maxGraphInstances caps how many instances one response serializes.
	maxGraphInstances = 1000
)

// graphBounds is how far instanceGraphToProto expands an object graph: nested
// objects to Depth, and Instances objects in all.
type graphBounds struct {
	Depth     int
	Instances int
}

// defaultGraphBounds are the bounds the service serializes an instance graph under.
func defaultGraphBounds() graphBounds {
	return graphBounds{Depth: maxGraphDepth, Instances: maxGraphInstances}
}

// instanceGraph is an object graph serialized by instanceGraphToProto.
type instanceGraph struct {
	Root *JInstance
	All  []*JInstance // the root first
	// Truncated reports that the instance bound cut the graph short.
	Truncated bool
	// Errors are the typed failures behind every FeatureValue.Error in All.
	Errors []error
}

// instanceGraphToProto converts inst and every instance reachable from it. The
// root is returned first; runtime instances live only for the duration of a
// request, so the whole reachable graph is serialized while the context is alive.
//
// Expansion stops at a child whose type is already on the path, at maxGraphDepth
// and at maxGraphInstances: reading a composite feature value materializes the object it
// holds, so a self-referential part would otherwise instantiate forever. An
// unexpanded child stays a bare instance id.
func instanceGraphToProto(rt *runtime.Context, inst *runtime.Instance, idx *symbols.Index) instanceGraph {
	bounds := defaultGraphBounds()
	var g instanceGraph
	seen := make(map[int64]bool)
	onPath := make(map[*symbols.Symbol]bool)

	var walk func(*runtime.Instance, int) *JInstance
	walk = func(cur *runtime.Instance, depth int) *JInstance {
		if seen[cur.ID] {
			return nil
		}
		if len(g.All) >= bounds.Instances {
			g.Truncated = true
			return nil
		}
		seen[cur.ID] = true
		onPath[cur.Type] = true
		defer delete(onPath, cur.Type)

		// instanceToProto reads every feature value through GetFeatureValue, which is what
		// lazily materializes the children the ids below resolve to.
		pbInst := instanceToProto(rt, cur, idx, func(err error) { g.Errors = append(g.Errors, err) })
		g.All = append(g.All, pbInst)

		if depth >= bounds.Depth {
			return pbInst
		}

		// In name order, so the graph is serialized in the same order every run.
		for _, name := range slices.Sorted(maps.Keys(pbInst.FeatureValues)) {
			for _, id := range instanceRefs(pbInst.FeatureValues[name]) {
				child, ok := rt.Instance(id)
				if !ok || onPath[child.Type] {
					continue
				}
				walk(child, depth+1)
			}
		}
		return pbInst
	}

	g.Root = walk(inst, 0)
	return g
}

// instanceRefs collects the instance IDs a feature value references, scalar or not.
func instanceRefs(fv *JFeatureValue) []int64 {
	var ids []int64
	var collect func(*JValue)
	collect = func(v *JValue) {
		if v.InstanceId != nil {
			ids = append(ids, int64(*v.InstanceId))
		}
		for _, nested := range nestedValues(v) {
			collect(nested)
		}
	}
	if fv.Value != nil {
		collect(fv.Value)
	}
	for _, v := range fv.Values {
		collect(v)
	}
	return ids
}

// nestedValues lists the Values a value holds directly: a sequence's or a set's
// elements, an array's elements and a vector's components.
func nestedValues(pv *JValue) []*JValue {
	switch {
	case pv.Sequence != nil:
		return pv.Sequence.Elements
	case pv.Set != nil:
		return pv.Set.Elements
	case pv.Array != nil:
		return pv.Array.Elements
	case pv.Vector != nil:
		return pv.Vector.Components
	}
	return nil
}

// instanceToProto converts runtime.Instance to the Instance message. Feature values
// are read through Instance.GetFeatureValue, so a derived default is evaluated against
// the instance rather than reported as unmaterialized. Each feature value it could
// not read is handed to failed as the runtime's typed error before its text is
// reported. Features are read in name order, because reading one materializes the
// object it holds, so map order would decide the ids those objects are given.
func instanceToProto(rt *runtime.Context, inst *runtime.Instance, idx *symbols.Index, failed func(error)) *JInstance {
	pbValues := make(map[string]*JFeatureValue)

	for _, name := range slices.Sorted(maps.Keys(inst.FeatureValues)) {
		fv, err := inst.GetFeatureValue(rt, name)
		if err != nil {
			failed(err)
			pbValues[name] = &JFeatureValue{
				FeatureName: name,
				Error:       err.Error(),
			}
			continue
		}

		pbValue := &JFeatureValue{
			FeatureName:  name,
			Materialized: fv.Materialized,
		}

		// Check multiplicity to determine single- vs multi-valued
		if fv.Feature.Scalar() {
			// Single-valued. An unmaterialized feature value carries no value.
			switch {
			case !fv.Materialized:
			case fv.Value.Kind == runtime.ValInvalid:
				pbValue.Value = &JValue{Unset: boolean(true)}
			default:
				pbValue.Value = valueToProto(rt, fv.Value, idx)
			}
		} else {
			for _, elem := range objref.CollectionElements(fv.Values) {
				pbValue.Values = append(pbValue.Values, valueToProto(rt, elem, idx))
			}
		}

		pbValues[name] = pbValue
	}

	return &JInstance{
		Id:            I64(inst.ID),
		TypeSymbolId:  idx.GetFQN(inst.Type),
		FeatureValues: pbValues,
	}
}
