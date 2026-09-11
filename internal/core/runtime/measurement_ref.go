package runtime

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// MeasurementRef is a ScalarMeasurementReference held as a value: the Unit a
// quantity carries (named product = declarations, term = reduction), by itself.
type MeasurementRef struct {
	Unit Unit
}

// NewMeasurementRefValue wraps a unit as a measurement reference value.
func NewMeasurementRefValue(unit Unit) Value {
	return Value{Kind: ValMeasurementRef, ref: &MeasurementRef{Unit: unit}}
}

// measurementRefOf is the reference of a unit, spelled by its product when the
// notation gave it no text of its own.
func measurementRefOf(unit Unit) Value {
	if unit.Text == "" && !unit.Product.IsEmpty() {
		unit.Text = unit.Product.String()
	}
	return NewMeasurementRefValue(unit)
}

// Declaration is the one unit declaration the reference names at the first
// power (`SI::m`); nil for a unit composed of several (`m/s`, `m**2`).
func (r *MeasurementRef) Declaration() *symbols.Symbol {
	if r == nil || len(r.Unit.Product.Powers) != 1 {
		return nil
	}
	power := r.Unit.Product.Powers[0]
	if power.Exponent != 1 {
		return nil
	}
	return power.Unit
}

// String renders the reference as a quantity's unit is written: `m`, `km/h`, `m**2`.
func (r *MeasurementRef) String() string {
	if r == nil {
		return unknownText
	}
	if r.Unit.Product.IsEmpty() {
		return r.Unit.Term.String()
	}
	return r.Unit.String()
}

// equal holds for one reduction at one scale (`SI::'m/s' == m/s`, `km/m == m/mm`);
// a named dimension-one unit reduces to nothing, so it is only itself (`rad != sr`).
func (r *MeasurementRef) equal(other *MeasurementRef) bool {
	if r == nil || other == nil {
		return r == other
	}
	if !r.Unit.Term.Same(other.Unit.Term) {
		return false
	}
	if r.namesDimensionOne() || other.namesDimensionOne() {
		return r.Unit.Product.Equal(other.Unit.Product)
	}
	return true
}

// key identifies the reference the way equal compares it, for a set or map: the
// scale as one number, so `0.5` and `1/2` key alike as they compare alike.
func (r *MeasurementRef) key() string {
	if r == nil {
		return ""
	}
	scale := r.Unit.Term.Scale
	key := r.Unit.Term.DimensionKey() + "@" + strconv.FormatFloat(scale.Num/scale.Den, 'g', -1, 64)
	if r.namesDimensionOne() {
		key += "|" + r.Unit.Product.String()
	}
	return key
}

// namesDimensionOne reports a unit of dimension one by name (`rad`, `one`), not a
// ratio that cancels to a number (`km/m`).
func (r *MeasurementRef) namesDimensionOne() bool {
	return r.Unit.Term.Dimensionless() && r.Unit.Product.NamesDimensionOne()
}

// MeasurementUnitValue is the reference a measurement-unit declaration names
// (true for one). A library unit is the declaration whatever defines it (`'m/s'
// = m/s`); a model feature typed by a unit and given a value holds that value
// instead (false). A measurement scale (`UTC`, `'°C_abs'`) is the one-axis
// frame its declaration describes.
func (ctx *Context) MeasurementUnitValue(sym *symbols.Symbol) (Value, bool, error) {
	if sym == nil || ctx.model.semantics == nil {
		return Value{}, false, nil
	}
	if !ctx.model.semantics.IsMeasurementUnit(sym) {
		return ctx.measurementScaleValue(sym)
	}
	if !ctx.libraryDeclared(sym) && ctx.extractDefaultValue(sym) != nil {
		return Value{}, false, nil
	}
	term, err := ctx.model.semantics.UnitTermOf(sym)
	if err != nil {
		return Value{}, true, fmt.Errorf("%w: %s: %w", ErrNotAQuantity, sym.Name, err)
	}
	return DeclaredMeasurementRef(sym, "", term), true, nil
}

// DeclaredMeasurementRef is the reference a unit declaration names, reducing to
// term, spelt as text (`SI::km`) or, given none, by its symbol (`km`).
func DeclaredMeasurementRef(sym *symbols.Symbol, text string, term semantics.UnitTerm) Value {
	if text == "" {
		text = unitSymbolName(sym)
	}
	product := semantics.NamedUnitProduct(sym, text, term.Dimensionless())
	return NewMeasurementRefValue(Unit{Text: text, Product: product, Term: term})
}

// measurementScaleValue is the scale a measurement scale declaration (`SI::'°C_abs'`,
// `Time::UTC`) evaluates to: its object, read as the one-axis frame it is.
func (ctx *Context) measurementScaleValue(sym *symbols.Symbol) (Value, bool, error) {
	if !ctx.model.semantics.IsMeasurementScale(sym) {
		return Value{}, false, nil
	}
	if !ctx.libraryDeclared(sym) && ctx.extractDefaultValue(sym) != nil {
		return Value{}, false, nil
	}
	inst, err := ctx.occurrenceOf(sym)
	if err != nil {
		return Value{}, true, fmt.Errorf("measurement scale %s: %w", ctx.qualifiedUnitName(sym), err)
	}
	frame, err := ctx.readFrame(inst)
	if err != nil {
		return Value{}, true, err
	}
	return NewCoordinateFrameValue(frame), true, nil
}

// qualifiedUnitName names a reference declaration as it is written (`SI::'°C_abs'`,
// `Time::UTC`): its owner's qualified name and the symbol it is spelled by.
func (ctx *Context) qualifiedUnitName(sym *symbols.Symbol) string {
	name := unitSymbolName(sym)
	if sym.OwnerScope == nil || sym.OwnerScope.Owner() == nil {
		return name
	}
	return ctx.qualifiedSymbolName(sym.OwnerScope.Owner()) + "::" + name
}

// unitSymbolName is the symbol a unit is written by (`km`), its name otherwise.
func unitSymbolName(sym *symbols.Symbol) string {
	if sym.ShortName != "" {
		return lexer.NameText(sym.ShortName)
	}
	return lexer.NameText(sym.Name)
}

// composeMeasurementRefs is the unit a `*`, `/` or `**`/`^` (Real exponent) of
// references names, as MeasurementRefCalculations declares; false for other
// operands. A measurement scale is no unit factor: composing one is an error.
func (ctx *Context) composeMeasurementRefs(op ast.OperatorKind, left, right Value) (Value, bool, error) {
	if left.Kind != ValMeasurementRef {
		return Value{}, false, nil
	}
	x := left.MeasurementRef().Unit
	var result Unit
	switch op {
	case ast.OpMul, ast.OpDiv:
		if right.Kind != ValMeasurementRef {
			return Value{}, false, nil
		}
		y := right.MeasurementRef().Unit
		if err := ctx.refuseScaleFactor(op, x, y); err != nil {
			return Value{}, true, err
		}
		if op == ast.OpMul {
			result = Unit{Product: x.Product.Times(y.Product), Term: x.Term.Times(y.Term)}
		} else {
			result = Unit{Product: x.Product.DividedBy(y.Product), Term: x.Term.DividedBy(y.Term)}
		}
	case ast.OpPow:
		if right.Kind != ValConst || !right.Const.IsNumeric() {
			return Value{}, false, nil
		}
		if err := ctx.refuseScaleFactor(op, x); err != nil {
			return Value{}, true, err
		}
		exponent := right.Const.AsReal()
		result = Unit{Product: x.Product.Pow(exponent), Term: x.Term.Pow(exponent)}
	default:
		return Value{}, false, nil
	}
	return measurementRefOf(result), true, nil
}

// refuseScaleFactor is the error for composing a unit from a measurement scale.
func (ctx *Context) refuseScaleFactor(op ast.OperatorKind, units ...Unit) error {
	if ctx == nil || ctx.model == nil {
		return nil
	}
	for _, unit := range units {
		if scale, ok := ctx.model.semantics.MeasurementScaleOf(unit.Term); ok {
			return fmt.Errorf("%w: MeasurementRefCalculations::'%s': %s is a measurement scale, whose points are on the scale and not in a unit to compose",
				ErrUnevaluableLibraryFunction, op.String(), unitSymbolName(scale))
		}
	}
	return nil
}

// Library types and features a measurement reference or scalar quantity answers.
const (
	scalarMRefTypeFQN        = "MeasurementReferences::ScalarMeasurementReference"
	derivedUnitFQN           = "MeasurementReferences::DerivedUnit"
	scalarQuantityTypeFQN    = "Quantities::ScalarQuantityValue"
	mRefIsBoundFeature       = "isBound"
	mRefMRefsFeature         = "mRefs"
	mRefOrthogonalFeature    = "isOrthogonal"
	tensorContravariantOrder = "contravariantOrder"
	tensorCovariantOrder     = "covariantOrder"
)

// measurementRefFeature answers what a scalar reference fixes from the value; any other
// member reads from the named declaration's object, a typed error for a composed unit.
func (ctx *Context) measurementRefFeature(val Value, name string) (Value, bool, error) {
	ref := val.MeasurementRef()
	switch name {
	case arrayDimensionsFeature:
		return ctx.answerSequence(nil)
	case arrayRankFeature, tensorOrderFeature:
		return integerValue(0), true, nil
	case arrayFlattenedSizeFeature:
		return integerValue(1), true, nil
	case arrayElementsFeature, mRefMRefsFeature:
		return ctx.answerSequence([]Value{val})
	case mRefIsBoundFeature:
		return boolValue(false), true, nil
	case mRefOrthogonalFeature:
		return boolValue(true), true, nil
	}
	decl := ref.Declaration()
	if decl == nil {
		if !ctx.libraryTypeDeclares(derivedUnitFQN, name) {
			return Value{}, false, nil
		}
		return Value{}, true, fmt.Errorf(
			"%w: %s::%s: %s is a %s reducing to %s, which names no declaration whose member %s could be read",
			ErrUnevaluableLibraryFunction, derivedUnitFQN, name, ref, derivedUnitFQN, ref.Unit.Term, name,
		)
	}
	if member, ok, err := ctx.declarationMember(decl, name); ok || err != nil {
		return member, ok, err
	}
	if _, ok := ctx.model.semantics.LookupMember(decl, name); !ok {
		return Value{}, false, nil
	}
	// A valued member the unit's own declaration states (`K.temperatureOfWaterAtTriplePointInK`)
	// is read as a declaration of its own; one without a value (`km.unitConversion`) is not held.
	if decl := ref.Declaration(); decl != nil {
		if member, ok := ctx.ownMember(decl, name); ok {
			val, err := ctx.declaredMemberValue(member)
			if !errors.Is(err, ErrNoValue) {
				return val, true, err
			}
		}
	}
	return Value{}, true, fmt.Errorf(
		"%w: %s::%s: a measurement reference value holds the unit %s and its reduction %s, not the declaration's member %s",
		ErrUnevaluableLibraryFunction, scalarMRefTypeFQN, name, ref, ref.Unit.Term, name,
	)
}

// declarationMember reads a member of the object a declaration materializes as,
// the one its occurrence is. Reports whether the object carries the member.
func (ctx *Context) declarationMember(decl *symbols.Symbol, name string) (Value, bool, error) {
	inst, err := ctx.occurrenceOf(decl)
	if err != nil {
		return Value{}, true, fmt.Errorf("usage %s: %w", symbolText(decl), err)
	}
	if _, ok := inst.FeatureValues[name]; !ok {
		return Value{}, false, nil
	}
	fv, err := inst.GetFeatureValue(ctx, name)
	if err != nil {
		return Value{}, true, err
	}
	member, err := ctx.readFeatureValue(fv, name)
	return member, true, err
}

// unitObjectValue is the reference a unit declaration's object reads as: the object
// holds the declaration's members, the reference is what the declaration names.
func (ctx *Context) unitObjectValue(inst *Instance) (Value, bool, error) {
	if !ctx.declaresUnit(inst.Type) {
		return Value{}, false, nil
	}
	return ctx.MeasurementUnitValue(inst.Type)
}

// declaresUnit reports whether sym declares a measurement unit: a usage typed by a
// unit definition (`attribute <m> metre : LengthUnit`), which a reference names.
func (ctx *Context) declaresUnit(sym *symbols.Symbol) bool {
	return sym != nil && sym.Kind == symbols.SymbolAttributeUsage && ctx.model.semantics != nil && ctx.model.semantics.IsMeasurementUnit(sym)
}

// libraryTypeDeclares reports whether the loaded library type has a member name.
func (ctx *Context) libraryTypeDeclares(fqn, name string) bool {
	typ := ctx.librarySymbol(fqn)
	if typ == nil {
		return false
	}
	_, ok := ctx.model.semantics.LookupMember(typ, name)
	return ok
}

// ownMember is the member decl's own body declares, not one its type contributes.
func (ctx *Context) ownMember(decl *symbols.Symbol, name string) (*symbols.Symbol, bool) {
	member, ok := ctx.model.semantics.LookupMember(decl, name)
	if !ok || member.OwnerScope == nil || member.OwnerScope.Owner() != decl {
		return nil, false
	}
	return member, true
}

// declaredMemberValue reads a declaration as a name of it does: the one object
// its body describes, else the value it binds.
func (ctx *Context) declaredMemberValue(member *symbols.Symbol) (Value, error) {
	if ctx.namesOneObject(member) {
		inst, err := ctx.occurrenceOf(member)
		if err != nil {
			return Value{}, fmt.Errorf("usage %s: %w", symbolText(member), err)
		}
		return ctx.objectValue(inst)
	}
	return ctx.EvalDeclaredValue(member)
}

// quantityFeature answers a scalar quantity's `num`, `mRef` and the tensor and
// Array features a scalar fixes (order 0, one element); isBound is not held.
func (ctx *Context) quantityFeature(val Value, name string) (Value, bool, error) {
	q := val.Quantity()
	switch name {
	case vectorQuantityNumFeature:
		return Value{Kind: ValConst, Const: q.Num}, true, nil
	case vectorQuantityMRefFeature:
		// A point on a scale (`3 [UTC]`) is over the scale, not a unit.
		if scale, ok, err := ctx.scaleOfUnit(q.Unit); ok || err != nil {
			if err != nil {
				return Value{}, true, err
			}
			return NewCoordinateFrameValue(scale), true, nil
		}
		return measurementRefOf(q.Unit), true, nil
	case arrayDimensionsFeature:
		return ctx.answerSequence(nil)
	case arrayRankFeature, tensorOrderFeature, tensorContravariantOrder, tensorCovariantOrder:
		return integerValue(0), true, nil
	case arrayFlattenedSizeFeature:
		return integerValue(1), true, nil
	case arrayElementsFeature:
		return ctx.answerSequence([]Value{{Kind: ValConst, Const: q.Num}})
	case mRefIsBoundFeature:
		return Value{}, true, fmt.Errorf(
			"%w: %s::isBound: a scalar quantity value holds its num and mRef, not whether the quantity is bound",
			ErrUnevaluableLibraryFunction, scalarQuantityTypeFQN,
		)
	}
	return Value{}, false, nil
}

// vectorQuantityMRef is the frame a vector quantity was written over, else the one
// reference its axes share, however spelt; axes in different units over no frame
// have no reference faithful to all, a typed error.
func vectorQuantityMRef(vq *VectorQuantity) (Value, error) {
	if vq.Frame != nil {
		return NewCoordinateFrameValue(vq.Frame), nil
	}
	unit, ok := vq.sharedUnit()
	if !ok {
		return Value{}, fmt.Errorf(
			"%w: Quantities::VectorQuantityValue::mRef: the axes of %s carry different units, and no one measurement reference names them all",
			ErrUnevaluableLibraryFunction, vq.format(semantics.FormatConst),
		)
	}
	return measurementRefOf(unit), nil
}

// measurementRefValueType is the declaration's type (`LengthUnit`), or
// DerivedUnit for a unit composed of powers of several declarations.
func (ctx *Context) measurementRefValueType(ref *MeasurementRef) (*symbols.Symbol, error) {
	if decl := ref.Declaration(); decl != nil {
		if typ := ctx.extractType(decl); typ != nil {
			return typ, nil
		}
	}
	return ctx.loadedLibraryType(derivedUnitFQN)
}

// measurementRefConforms judges a reference written to a feature of the declared
// type as the checker does: by its type, or by dimension against a unit definition.
func (ctx *Context) measurementRefConforms(ref *MeasurementRef, declared *symbols.Symbol) (bool, string, error) {
	typ, err := ctx.measurementRefValueType(ref)
	if err != nil {
		return false, "", err
	}
	c := ctx.model.semantics.MeasurementRefConforms(typ, ref.Unit.Term, declared)
	if !c.Known || c.Holds {
		return true, "", nil
	}
	return false, fmt.Sprintf("cannot write the measurement reference %s, %s, to a feature typed by %s",
		ref, c.Found, symbolText(declared)), nil
}
