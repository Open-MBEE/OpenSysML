package runtime

import (
	"math"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// classifiedBy is which of a value's types a classification reads (KerML 1.0 Table 5):
// `hastype` the direct types alone; `istype`, `@`, `as` and a feature write those and their supertypes.
type classifiedBy uint8

const (
	byAnyType classifiedBy = iota
	byOwnType
)

// classifyValue is the one reading of "the value is of type T" shared by `as`, `istype`,
// `hastype`, `@` and feature writes: the value's declared and representation types decide it,
// and a target narrower than all of them is left to what the value itself states.
func (ctx *Context) classifyValue(
	scope *symbols.Scope, value Value, target *symbols.Symbol, declared []*symbols.Symbol, by classifiedBy,
) (semantics.TypeClassification, error) {
	return ctx.classifyValueReading(scope, value, target, declared, by, nil)
}

// classifyValueReading answers classifyValue; reading holds the composed targets
// being read, so a composition naming itself does not recur.
func (ctx *Context) classifyValueReading(
	scope *symbols.Scope, value Value, target *symbols.Symbol, declared []*symbols.Symbol,
	by classifiedBy, reading map[*symbols.Symbol]bool,
) (semantics.TypeClassification, error) {
	if isNaN(value) {
		// A NaN states no scalar type, so no type holds it whatever a feature declares.
		return semantics.ClassifiesNone, nil
	}
	types, err := ctx.valueTypes(scope, value)
	if by == byOwnType {
		// A direct type is the value's own; what a feature declares of it is not.
		if err != nil {
			return semantics.ClassifiesNone, err
		}
		return classifiesIf(slices.Contains(types, target)), nil
	}
	if err != nil && len(declared) == 0 {
		return semantics.ClassifiesNone, err
	}
	known := append(append([]*symbols.Symbol{}, declared...), types...)
	verdict := ctx.model.semantics.ClassifiesTypes(known, target)
	if verdict == semantics.ClassifiesNone && isScalar(value) {
		// A scalar type beside the value's own (`Cost :> Real` beside Rational) may hold it.
		if decided, ok := ctx.representationClassifies(value, target); ok {
			return decided, nil
		}
	}
	if verdict != semantics.ClassifiesSome {
		return verdict, nil
	}
	if composed, ok, err := ctx.classifyComposed(scope, value, target, declared, reading); ok {
		return composed, err
	}
	return ctx.classifyNarrower(scope, value, target)
}

// classifyNarrower decides a target narrower than every type the value is of, which only
// the value itself can: a scalar by its representation, a quantity by its dimension, a
// structured value by its shape and units; an object or enumeration literal is of the
// types it carries and no narrower; `5` against `Even` stays undecided.
func (ctx *Context) classifyNarrower(
	scope *symbols.Scope, value Value, target *symbols.Symbol,
) (semantics.TypeClassification, error) {
	if isScalar(value) {
		if decided, ok := ctx.representationClassifies(value, target); ok {
			return decided, nil
		}
		return semantics.ClassifiesSome, nil
	}
	switch value.Kind {
	case ValQuantity:
		return ctx.quantityClassifies(value, target), nil
	case ValArray, ValVector, ValVectorQuantity, ValTensorQuantity,
		ValMeasurementRef, ValCoordinateFrame, ValCoordinateTransformation:
		keep, _, err := ctx.valueConforms(scope, &value, target, admitWritten)
		if err != nil {
			return semantics.ClassifiesNone, err
		}
		return classifiesIf(keep), nil
	case ValEnumLiteral, ValVariant, ValInstance:
		return semantics.ClassifiesNone, nil
	}
	return semantics.ClassifiesSome, nil
}

// isScalar reports a value whose representation states a ScalarValues type.
func isScalar(value Value) bool {
	switch value.Kind {
	case ValConst, ValComplex, ValString:
		return true
	}
	return false
}

// classifiesIf is the verdict a yes-or-no judgement amounts to.
func classifiesIf(yes bool) semantics.TypeClassification {
	if yes {
		return semantics.ClassifiesAll
	}
	return semantics.ClassifiesNone
}

// valueTypes names the types a value is of by itself: a deferred expression its
// evaluation type, a quantity the scalar quantity type, a scalar its library type.
func (ctx *Context) valueTypes(scope *symbols.Scope, value Value) ([]*symbols.Symbol, error) {
	switch value.Kind {
	case ValExpr:
		if typ := ctx.model.semantics.ExprResultType(value.exprScope(scope), value.Expr()); typ != nil {
			return []*symbols.Symbol{typ}, nil
		}
		evaluation, err := ctx.loadedLibraryType(evaluationTypeFQN)
		if err != nil {
			return nil, err
		}
		return []*symbols.Symbol{evaluation}, nil
	case ValQuantity:
		quantity, err := ctx.loadedLibraryType(scalarQuantityTypeFQN)
		if err != nil {
			return nil, err
		}
		return []*symbols.Symbol{quantity}, nil
	}
	// The library symbol answers ahead of any same-named declaration in scope.
	if scalar := ctx.scalarLibraryType(value); scalar != nil {
		return ctx.numberTypes(value, scalar), nil
	}
	return ctx.directValueTypes(scope, value)
}

// numberTypes adds to a number's scalar type the quantity type it also is: the runtime
// holds a quantity of dimension one and no unit as the bare number (ToDimensionOneValue).
func (ctx *Context) numberTypes(value Value, scalar *symbols.Symbol) []*symbols.Symbol {
	types := []*symbols.Symbol{scalar}
	if value.Kind != ValConst || (value.Const.Kind != semantics.ValInt && value.Const.Kind != semantics.ValReal) {
		return types
	}
	if dimOne := ctx.librarySymbol(dimensionOneValueFQN); dimOne != nil {
		types = append(types, dimOne)
	}
	return types
}

// scalarLibraryType is the ScalarValues type a scalar's representation states
// (representationPrim), or nil for a value that is no scalar or is NaN.
func (ctx *Context) scalarLibraryType(value Value) *symbols.Symbol {
	prim := representationPrim(value)
	if prim == semantics.PrimUnknown {
		return nil
	}
	return ctx.librarySymbol(semantics.ScalarFQN(prim))
}

// representationClassifies reads a scalar target narrower than a scalar's types off its
// representation (KerML 1.0 §8.4.4.9.2): a finite real is a Rational and no Integer
// whatever number it holds; an integer is an Integer; `*` is a Positive (§8.4.4.6). A
// target the lattice does not place (`Natural`, `Positive`, a subtype a model declares)
// marks no evaluated value, so it stays undecided unless its nearest lattice ancestor or
// the Natural and Positive bounds (§9.3.2.2.4, §9.3.2.2.7) exclude the value. False
// second for a target above no scalar.
func (ctx *Context) representationClassifies(value Value, target *symbols.Symbol) (semantics.TypeClassification, bool) {
	prim := ctx.model.semantics.PrimTypeOf(target)
	if prim == semantics.PrimUnknown {
		return semantics.ClassifiesNone, false
	}
	got := representationPrim(value)
	if value.Kind == ValConst && value.Const.IsUnbounded() {
		// `*` is the Positive exceeding every bound (KerML 8.4.4.6), so its lattice place decides.
		got = semantics.PrimNatural
	}
	switch {
	case got == semantics.PrimUnknown:
		return semantics.ClassifiesNone, true
	case got == semantics.PrimInteger && prim == semantics.PrimNatural:
		if value.Const.Int < 0 || value.Const.Int == 0 && ctx.positiveScalar(target) {
			return semantics.ClassifiesNone, true
		}
		return semantics.ClassifiesSome, true
	case !semantics.PrimConforms(got, prim):
		return semantics.ClassifiesNone, true
	}
	if _, exact := ctx.model.semantics.ScalarLatticeElement(target); exact {
		return semantics.ClassifiesAll, true
	}
	return semantics.ClassifiesSome, true
}

// representationPrim is the lattice element a scalar's representation states: an integer an
// Integer, a finite real a Rational, an infinite one a Real, a complex a Complex whatever
// its imaginary part, a NaN nothing.
func representationPrim(value Value) semantics.PrimType {
	switch value.Kind {
	case ValString:
		return semantics.PrimString
	case ValComplex:
		z := value.Complex()
		if math.IsNaN(real(z)) || math.IsNaN(imag(z)) {
			return semantics.PrimUnknown
		}
		return semantics.PrimComplex
	case ValConst:
		switch value.Const.Kind {
		case semantics.ValBool:
			return semantics.PrimBoolean
		case semantics.ValInt:
			return semantics.PrimInteger
		case semantics.ValReal:
			return realRepresentationPrim(value.Const.Real)
		}
	}
	return semantics.PrimUnknown
}

// isNaN reports a real or complex value that is not a number.
func isNaN(value Value) bool {
	switch value.Kind {
	case ValComplex:
		z := value.Complex()
		return math.IsNaN(real(z)) || math.IsNaN(imag(z))
	case ValConst:
		return value.Const.Kind == semantics.ValReal && math.IsNaN(value.Const.Real)
	}
	return false
}

func realRepresentationPrim(x float64) semantics.PrimType {
	switch {
	case math.IsNaN(x):
		return semantics.PrimUnknown
	case math.IsInf(x, 0):
		return semantics.PrimReal
	}
	return semantics.PrimRational
}

// quantityClassifies reads a quantity type narrower than ScalarQuantityValue off the
// quantity's dimension: one the target fixes and the unit is commensurable with holds
// it, an incommensurable one does not, and a type fixing no dimension stays undecided.
func (ctx *Context) quantityClassifies(value Value, target *symbols.Symbol) semantics.TypeClassification {
	want, ok := ctx.model.semantics.DimensionOfType(target)
	if !ok || value.Quantity() == nil {
		return semantics.ClassifiesSome
	}
	got, ok := ctx.model.semantics.DimensionOfUnit(value.Quantity().Unit.Term)
	if !ok {
		return semantics.ClassifiesSome
	}
	if !want.Term.Commensurable(got.Term) {
		return semantics.ClassifiesNone
	}
	if !ctx.model.semantics.FixesMeasurementReference(target) {
		return semantics.ClassifiesSome
	}
	return semantics.ClassifiesAll
}

// classifyComposed decides a value by a composed target's operands — any of a union, all of an
// intersection, the first of a difference and none of the rest — reporting second whether target is composed.
func (ctx *Context) classifyComposed(
	scope *symbols.Scope, value Value, target *symbols.Symbol, declared []*symbols.Symbol,
	reading map[*symbols.Symbol]bool,
) (semantics.TypeClassification, bool, error) {
	unions := ctx.model.semantics.UnioningTypes(target)
	intersects := ctx.model.semantics.IntersectingTypes(target)
	differences := ctx.model.semantics.DifferencingTypes(target)
	if len(unions)+len(intersects)+len(differences) == 0 || reading[target] {
		return semantics.ClassifiesNone, false, nil
	}
	if reading == nil {
		reading = make(map[*symbols.Symbol]bool)
	}
	reading[target] = true
	defer delete(reading, target)

	classifies := func(operand *symbols.Symbol) (semantics.TypeClassification, error) {
		return ctx.classifyValueReading(scope, value, operand, declared, byAnyType, reading)
	}
	// An undecided operand leaves the whole undecided unless another excludes the value outright.
	undecided := false
	if len(unions) > 0 {
		verdict, err := anyClassifies(unions, classifies)
		if err != nil {
			return semantics.ClassifiesNone, true, err
		}
		switch verdict {
		case semantics.ClassifiesNone:
			return semantics.ClassifiesNone, true, nil
		case semantics.ClassifiesSome:
			undecided = true
		}
	}
	for _, operand := range intersects {
		verdict, err := classifies(operand)
		if err != nil {
			return semantics.ClassifiesNone, true, err
		}
		switch verdict {
		case semantics.ClassifiesNone:
			return semantics.ClassifiesNone, true, nil
		case semantics.ClassifiesSome:
			undecided = true
		}
	}
	for i, operand := range differences {
		verdict, err := classifies(operand)
		if err != nil {
			return semantics.ClassifiesNone, true, err
		}
		switch {
		case verdict == semantics.ClassifiesSome:
			undecided = true
		case (verdict == semantics.ClassifiesAll) != (i == 0):
			return semantics.ClassifiesNone, true, nil
		}
	}
	if undecided {
		return semantics.ClassifiesSome, true, nil
	}
	return semantics.ClassifiesAll, true, nil
}

// anyClassifies is a union's verdict: any operand classifying the value settles it,
// an undecided operand counts only where none does.
func anyClassifies(
	operands []*symbols.Symbol, classifies func(*symbols.Symbol) (semantics.TypeClassification, error),
) (semantics.TypeClassification, error) {
	out := semantics.ClassifiesNone
	for _, operand := range operands {
		verdict, err := classifies(operand)
		if err != nil {
			return semantics.ClassifiesNone, err
		}
		if verdict == semantics.ClassifiesAll {
			return semantics.ClassifiesAll, nil
		}
		if verdict == semantics.ClassifiesSome {
			out = semantics.ClassifiesSome
		}
	}
	return out, nil
}
