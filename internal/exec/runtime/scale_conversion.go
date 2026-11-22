package runtime

import (
	"fmt"
	"math"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// scaleAnchor places a scale's zero on the reference it is defined against: a
// magnitude x on the scale is `x [scale.unit]` converted to the source's
// difference unit, plus origin, a point on the source.
type scaleAnchor struct {
	source *CoordinateFrame
	origin Quantity
}

// differenceUnit is the unit two points on a one-axis reference differ in: a
// scale's declared unit, or the unit a ratio reference is.
func differenceUnit(frame *CoordinateFrame) Unit {
	if frame.isScale() {
		return frame.Scale.Unit
	}
	return frame.Axes[0]
}

// scaleOfUnit is the measurement scale a quantity's unit anchors to (`3 [UTC]`),
// false for a unit.
func (ctx *Context) scaleOfUnit(unit Unit) (*CoordinateFrame, bool, error) {
	decl, ok := ctx.model.semantics.MeasurementScaleOf(unit.Term)
	if !ok {
		return nil, false, nil
	}
	val, ok, err := ctx.measurementScaleValue(decl)
	if err != nil || !ok || val.Kind != ValCoordinateFrame {
		return nil, ok, err
	}
	return val.CoordinateFrame(), true, nil
}

// scaleName names a scale as its declaration is written (`SI::'°C_abs'`).
func (ctx *Context) scaleName(scale *CoordinateFrame) string {
	if scale.Decl != nil {
		return ctx.qualifiedUnitName(scale.Decl)
	}
	return scale.Name()
}

// anchorOf is where the scale's zero lies: from its CoordinateFramePlacement, from
// its quantityValueMapping, or from both when they agree; a scale stating neither
// converts to nothing outside itself.
func (ctx *Context) anchorOf(scale *CoordinateFrame) (*scaleAnchor, error) {
	name := ctx.scaleName(scale)
	placed, err := ctx.placementAnchor(name, scale)
	if err != nil {
		return nil, err
	}
	mapped, err := ctx.mappingAnchor(name, scale)
	if err != nil {
		return nil, err
	}
	switch {
	case placed == nil && mapped == nil:
		return nil, fmt.Errorf("%w: %s states neither a transformation placing it on another reference nor a quantityValueMapping, so its magnitudes convert to no other reference",
			errUnevaluableLibraryFunction, name)
	case placed == nil:
		return mapped, nil
	case mapped == nil:
		return placed, nil
	}
	mappedOrigin, err := semantics.ConvertQuantity(mapped.origin, placed.origin.Unit)
	if err != nil || !sameReference(placed.source, mapped.source) {
		return nil, fmt.Errorf("%w: %s: its transformation places zero on %s but its quantityValueMapping places it on %s",
			errUnevaluableLibraryFunction, name, placed.source.Name(), mapped.source.Name())
	}
	if !nearlyEqual(placed.origin.Num.AsReal(), mappedOrigin.Num.AsReal()) {
		return nil, fmt.Errorf("%w: %s: its transformation places zero at %s but its quantityValueMapping places it at %s",
			errUnevaluableLibraryFunction, name, placed.origin.String(), mappedOrigin.String())
	}
	return placed, nil
}

// placementAnchor reads the scale's CoordinateFramePlacement: a numeric origin on
// the source and, at most, a basis direction of magnitude one; nil without one.
func (ctx *Context) placementAnchor(name string, scale *CoordinateFrame) (*scaleAnchor, error) {
	t := scale.Transformation
	if t == nil {
		return nil, nil
	}
	if t.Placement == nil {
		return nil, fmt.Errorf("%w: %s: its transformation %s is %s; a scale is placed on its source by a CoordinateFramePlacement",
			errUnevaluableLibraryFunction, name, t.Name(), t.shapeName())
	}
	if t.Source == nil || t.Target == nil {
		return nil, fmt.Errorf("%w: %s: its transformation %s states no source or no target", ErrNoValue, name, t.Name())
	}
	if err := t.sameDimensions(name); err != nil {
		return nil, err
	}
	origin, ok := asQuantity(t.Placement.Origin)
	if !ok {
		return nil, fmt.Errorf("%w: %s: the origin of its transformation %s is %s, not a quantity on %s",
			errUnevaluableLibraryFunction, name, t.Name(), DescribeValue(t.Placement.Origin), t.Source.Name())
	}
	onSource, err := semantics.ConvertQuantity(*origin, t.Source.Axes[0])
	if err != nil {
		return nil, fmt.Errorf("%w: %s: the origin of its transformation %s: %w", ErrIncommensurableUnits, name, t.Name(), err)
	}
	if dirs := t.Placement.BasisDirections; len(dirs) > 1 {
		return nil, fmt.Errorf("%w: %s: its transformation %s states %d basisDirections over %s of one axis; a placement states none or one per axis",
			ErrMultiplicityViolation, name, t.Name(), len(dirs), t.Source.Name())
	}
	for _, dir := range t.Placement.BasisDirections {
		if err := identityBasis(name, t, dir); err != nil {
			return nil, err
		}
	}
	return &scaleAnchor{source: t.Source, origin: onSource}, nil
}

// identityBasis accepts the one basis direction a scale may state: a quantity on its
// source of magnitude one there; any other size scales the axis, which the library gives no meaning.
func identityBasis(name string, t *CoordinateTransformation, dir Value) error {
	q, ok := asQuantity(dir)
	if vq := dir.VectorQuantity(); !ok && vq != nil && vq.dimension() == 1 {
		if vq.Frame != nil && !vq.Frame.equal(t.Source) {
			return fmt.Errorf("%w: %s: the basisDirection of its transformation %s is a vector quantity in %s, not a quantity on %s, its source",
				ErrTypeMismatch, name, t.Name(), vq.Frame.Name(), t.Source.Name())
		}
		q, ok = vq.component(0), true
	}
	if !ok {
		return fmt.Errorf("%w: %s: a basisDirection of its transformation %s is %s, not a quantity on %s",
			errUnevaluableLibraryFunction, name, t.Name(), DescribeValue(dir), t.Source.Name())
	}
	onSource, err := semantics.ConvertQuantity(*q, t.Source.Axes[0])
	if err != nil {
		return fmt.Errorf("%w: %s: the basisDirection %s of its transformation %s is not on %s, its source: %w",
			ErrIncommensurableUnits, name, q.String(), t.Name(), t.Source.Name(), err)
	}
	if onSource.Num.AsReal() != 1 {
		return fmt.Errorf("%w: %s: the basisDirection %s of its transformation %s is not the identity 1 [%s], and the library gives a scale no other basis",
			errUnevaluableLibraryFunction, name, q.String(), t.Name(), t.Source.Name())
	}
	return nil
}

// mappingAnchor derives the zero from the quantityValueMapping: the reference
// point less the mapped point expressed in the reference's unit; nil without one.
func (ctx *Context) mappingAnchor(name string, scale *CoordinateFrame) (*scaleAnchor, error) {
	m := scale.Scale.Mapping
	if m == nil {
		return nil, nil
	}
	source, err := ctx.referenceFrameOfUnit(m.Reference.Unit)
	if err != nil {
		return nil, fmt.Errorf("%s: quantityValueMapping: %w", name, err)
	}
	mappedOnSource, err := semantics.ConvertQuantity(Quantity{Num: m.Mapped.Num, Unit: scale.Scale.Unit}, differenceUnit(source))
	if err != nil {
		return nil, fmt.Errorf("%w: %s: its quantityValueMapping maps %s to %s: %w",
			ErrIncommensurableUnits, name, m.Mapped.String(), m.Reference.String(), err)
	}
	num, err := semantics.MagnitudeArith(ast.OpSub, m.Reference.Num, mappedOnSource.Num, ctx.maxIntegerBits)
	if err != nil {
		return nil, err
	}
	return &scaleAnchor{source: source, origin: Quantity{Num: num, Unit: m.Reference.Unit}}, nil
}

// referenceFrameOfUnit is the reference a unit stands for as a scale's source:
// the scale it anchors to, else the one-axis frame of the unit itself.
func (ctx *Context) referenceFrameOfUnit(unit Unit) (*CoordinateFrame, error) {
	scale, ok, err := ctx.scaleOfUnit(unit)
	if err != nil {
		return nil, err
	}
	if ok {
		return scale, nil
	}
	return scalarFrame(measurementRefOf(unit).MeasurementRef()), nil
}

// sameReference holds when two frames name one reference: one frame, or one unit.
func sameReference(a, b *CoordinateFrame) bool {
	if a.equal(b) {
		return true
	}
	return !a.isScale() && !b.isScale() && len(a.Axes) == 1 && len(b.Axes) == 1 &&
		(&MeasurementRef{Unit: a.Axes[0]}).equal(&MeasurementRef{Unit: b.Axes[0]})
}

// nearlyEqual compares two placements of a zero up to binary64 rounding.
func nearlyEqual(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9*math.Max(1, math.Max(math.Abs(a), math.Abs(b)))
}

// toRatioReference expresses a quantity on a scale in the unit the scale is
// placed on, through every scale in between; a quantity in a unit is itself.
// A scale reached twice is a cycle, which reading the scales normally catches first.
func (ctx *Context) toRatioReference(q Quantity) (Quantity, error) {
	var passed []*CoordinateFrame
	for {
		scale, ok, err := ctx.scaleOfUnit(q.Unit)
		if err != nil {
			return Quantity{}, err
		}
		if !ok {
			return q, nil
		}
		for _, seen := range passed {
			if seen.equal(scale) {
				return Quantity{}, fmt.Errorf("%w: coordinate frame %s is defined in terms of itself", ErrCyclicFeatureValue, ctx.scaleName(scale))
			}
		}
		passed = append(passed, scale)
		switch ctx.scaleKindOf(scale.Type) {
		case ordinalScale:
			return Quantity{}, ctx.notIntervalError("conversion to another reference", scale)
		case cyclicRatioScale:
			if q, err = ctx.ratioMagnitude(q, scale); err != nil {
				return Quantity{}, err
			}
			continue
		case logarithmicScale:
			if q, err = ctx.fromLogarithm(q, scale); err != nil {
				return Quantity{}, err
			}
			continue
		}
		anchor, err := ctx.anchorOf(scale)
		if err != nil {
			return Quantity{}, err
		}
		onSource, err := semantics.ConvertQuantity(Quantity{Num: q.Num, Unit: scale.Scale.Unit}, differenceUnit(anchor.source))
		if err != nil {
			return Quantity{}, fmt.Errorf("%w: %s: its unit %s: %w", ErrIncommensurableUnits, ctx.scaleName(scale), scale.Scale.Unit, err)
		}
		num, err := semantics.MagnitudeArith(ast.OpAdd, onSource.Num, anchor.origin.Num, ctx.maxIntegerBits)
		if err != nil {
			return Quantity{}, err
		}
		q = Quantity{Num: num, Unit: anchor.origin.Unit}
	}
}

// onScale expresses a quantity as a point on the scale: first on the scale's
// source, then less the origin, in the scale's unit.
func (ctx *Context) onScale(q Quantity, scale *CoordinateFrame) (Quantity, error) {
	if current, ok, err := ctx.scaleOfUnit(q.Unit); err != nil {
		return Quantity{}, err
	} else if ok && current.equal(scale) {
		return q, nil
	} else if err := ctx.checkCommensurable(q, ctx.measuredUnit(scale)); err != nil {
		return Quantity{}, err
	}
	switch ctx.scaleKindOf(scale.Type) {
	case ordinalScale:
		return Quantity{}, ctx.notIntervalError("conversion from another reference", scale)
	case cyclicRatioScale:
		return ctx.toCyclicScale(q, scale)
	case logarithmicScale:
		return ctx.toLogarithm(q, scale)
	}
	anchor, err := ctx.anchorOf(scale)
	if err != nil {
		return Quantity{}, err
	}
	onSource, err := ctx.convertToReference(q, anchor.source)
	if err != nil {
		return Quantity{}, err
	}
	diff, err := semantics.MagnitudeArith(ast.OpSub, onSource.Num, anchor.origin.Num, ctx.maxIntegerBits)
	if err != nil {
		return Quantity{}, err
	}
	inUnit, err := semantics.ConvertQuantity(Quantity{Num: diff, Unit: differenceUnit(anchor.source)}, scale.Scale.Unit)
	if err != nil {
		return Quantity{}, fmt.Errorf("%w: %s: its unit %s: %w", ErrIncommensurableUnits, ctx.scaleName(scale), scale.Scale.Unit, err)
	}
	return Quantity{Num: inUnit.Num, Unit: scale.Axes[0]}, nil
}

// checkCommensurable rejects a quantity of another dimension than the unit before
// any scale is consulted: it lands on no scale, however the scale is anchored.
func (ctx *Context) checkCommensurable(q Quantity, unit Unit) error {
	from := q.Unit
	if scale, ok, err := ctx.scaleOfUnit(q.Unit); err != nil {
		return err
	} else if ok {
		from = ctx.measuredUnit(scale)
	}
	_, err := semantics.ConvertQuantity(Quantity{Num: q.Num, Unit: from}, unit)
	return err
}

// measuredUnit is a unit of the quantity a scale's values measure: the unit of a
// logarithmic scale's referenceQuantity, else the scale's declared unit.
func (ctx *Context) measuredUnit(scale *CoordinateFrame) Unit {
	log := scale.Scale.Log
	if log == nil || log.Reference == nil {
		return scale.Scale.Unit
	}
	if on, ok, err := ctx.scaleOfUnit(log.Reference.Unit); err == nil && ok {
		return on.Scale.Unit
	}
	return log.Reference.Unit
}

// unitFrame is the one-axis reference a unit is, as a conversion target.
func unitFrame(unit Unit) *CoordinateFrame {
	return scalarFrame(measurementRefOf(unit).MeasurementRef())
}

// toCyclicScale expresses a quantity on a cyclic ratio scale: its magnitude in the scale's unit.
func (ctx *Context) toCyclicScale(q Quantity, scale *CoordinateFrame) (Quantity, error) {
	if _, err := ctx.ratioMagnitude(q, scale); err != nil {
		return Quantity{}, err
	}
	inUnit, err := ctx.convertToReference(q, unitFrame(scale.Scale.Unit))
	if err != nil {
		return Quantity{}, err
	}
	return Quantity{Num: inUnit.Num, Unit: scale.Axes[0]}, nil
}

// noScaleValueError marks a quantity a well-formed logarithmic scale has no value for,
// or a value of one no quantity; a scale that is itself malformed is never this.
type noScaleValueError struct{ err error }

func (e noScaleValueError) Error() string { return e.err.Error() }
func (e noScaleValueError) Unwrap() error { return e.err }

// logReference is the ratio magnitude x_ref a logarithmic scale's values are relative to.
func (ctx *Context) logReference(scale *CoordinateFrame) (*Quantity, error) {
	name := ctx.scaleName(scale)
	ref := scale.Scale.Log.Reference
	if ref == nil {
		return nil, fmt.Errorf("%w: %s states no referenceQuantity, the quantity its values are logarithms relative to, so they convert to no other reference",
			errUnevaluableLibraryFunction, name)
	}
	if on, point, err := ctx.pointOf(ref); err != nil {
		return nil, err
	} else if point {
		return nil, fmt.Errorf("%w: %s: its referenceQuantity %s is on %s; a logarithm is of a ratio of magnitudes in a unit",
			errUnevaluableLibraryFunction, name, ref.String(), ctx.describeScale(on))
	}
	if !ref.Num.IsNumeric() || ref.Num.AsReal() == 0 {
		return nil, fmt.Errorf("%w: %s: its referenceQuantity %s is no divisor", semantics.ErrArithmeticDomain, name, ref.String())
	}
	return ref, nil
}

// fromLogarithm is the quantity x a value v on a logarithmic scale stands for,
// `v = factor * log_base((x / x_ref) ** exponent)` solved for the positive ratio.
func (ctx *Context) fromLogarithm(q Quantity, scale *CoordinateFrame) (Quantity, error) {
	ref, err := ctx.logReference(scale)
	if err != nil {
		return Quantity{}, err
	}
	log := scale.Scale.Log
	num, err := semantics.RealResult(scaledPower(ref.Num.AsReal(), log.Base, q.Num.AsReal()/log.Factor/log.Exponent))
	if err != nil {
		return Quantity{}, noScaleValueError{fmt.Errorf("%s as a quantity in %s: %w", q.String(), ref.Unit, err)}
	}
	return Quantity{Num: num, Unit: ref.Unit}, nil
}

// minNormal is the least positive binary64 with full precision.
const minNormal = 0x1p-1022

// inRange reports a positive ratio binary64 holds with full precision.
func inRange(ratio float64) bool { return ratio >= minNormal && !math.IsInf(ratio, 1) }

// scaledPower is ref * base**e; a power binary64 does not hold is taken with ref in the exponent.
func scaledPower(ref, base, e float64) float64 {
	if power := math.Pow(base, e); inRange(power) {
		return ref * power
	}
	return math.Copysign(math.Pow(base, e+logarithm(math.Abs(ref), base)), ref)
}

// logRatio is log_base(x / ref) for a positive ratio, as a difference where binary64 does not hold it.
func logRatio(x, ref, base float64) float64 {
	if ratio := x / ref; inRange(ratio) {
		return logarithm(ratio, base)
	}
	return logarithm(math.Abs(x), base) - logarithm(math.Abs(ref), base)
}

// logarithm is log_base(x), by the exact functions where the base has one.
func logarithm(x, base float64) float64 {
	if x > 0 && x < minNormal {
		// A subnormal is scaled into range first: math.Log is not accurate below it.
		return logarithm(math.Ldexp(x, 64), base) - 64*logarithm(2, base)
	}
	switch base {
	case 10:
		return math.Log10(x)
	case 2:
		return math.Log2(x)
	}
	return math.Log(x) / math.Log(base)
}

// toLogarithm is a quantity's value on a logarithmic scale:
// `factor * log_base((x / x_ref) ** exponent)`, defined where the ratio is positive.
func (ctx *Context) toLogarithm(q Quantity, scale *CoordinateFrame) (Quantity, error) {
	ref, err := ctx.logReference(scale)
	if err != nil {
		return Quantity{}, err
	}
	x, err := ctx.convertToReference(q, unitFrame(ref.Unit))
	if err != nil {
		return Quantity{}, err
	}
	log := scale.Scale.Log
	ratio := x.Num.AsReal() / ref.Num.AsReal()
	if x.Num.AsReal() == 0 || math.Signbit(x.Num.AsReal()) != math.Signbit(ref.Num.AsReal()) {
		// A ratio that is not positive has no one value the mapping returns it from.
		return Quantity{}, noScaleValueError{fmt.Errorf("%w: %s has no value on %s: its ratio to %s is %g, and a logarithm is of a positive ratio",
			semantics.ErrArithmeticDomain, q.String(), ctx.describeScale(scale), ref.String(), ratio)}
	}
	num, err := semantics.RealResult(log.Factor * log.Exponent * logRatio(x.Num.AsReal(), ref.Num.AsReal(), log.Base))
	if err != nil {
		return Quantity{}, noScaleValueError{err}
	}
	return Quantity{Num: num, Unit: scale.Axes[0]}, nil
}

// convertToReference is ConvertQuantity over a target that may be a scale, or a
// source that is: a quantity on a scale is carried to a ratio unit first.
func (ctx *Context) convertToReference(q Quantity, target *CoordinateFrame) (Quantity, error) {
	if target.isScale() {
		return ctx.onScale(q, target)
	}
	if err := ctx.checkCommensurable(q, target.Axes[0]); err != nil {
		return Quantity{}, err
	}
	ratio, err := ctx.toRatioReference(q)
	if err != nil {
		return Quantity{}, err
	}
	converted, err := semantics.ConvertQuantity(ratio, target.Axes[0])
	if err != nil || converted.Num.Kind != semantics.ValReal {
		return converted, err
	}
	// A magnitude the target unit puts past binary64 is an overflow, not an infinite quantity.
	num, err := semantics.RealResult(converted.Num.Real)
	return Quantity{Num: num, Unit: converted.Unit}, err
}

// conversionTarget reads ConvertQuantity's targetMRef: a unit or a scale, as the
// one-axis frame either is; a frame of more axes is no scalar reference.
func (ctx *Context) conversionTarget(name string, val Value) (*CoordinateFrame, error) {
	val = soleElement(val)
	switch val.Kind {
	case ValMeasurementRef:
		return scalarFrame(val.MeasurementRef()), nil
	case ValCoordinateFrame:
		frame := val.CoordinateFrame()
		if frame.isScale() {
			return frame, nil
		}
		return nil, fmt.Errorf("%w: function %s parameter %q requires a scalar measurement reference such as SI::m, got the coordinate frame %s of %d axes",
			ErrTypeMismatch, name, "targetMRef", frame.Name(), len(frame.Axes))
	}
	return nil, fmt.Errorf("%w: function %s parameter %q requires a measurement reference such as SI::m, got %s",
		ErrTypeMismatch, name, "targetMRef", DescribeValue(val))
}
