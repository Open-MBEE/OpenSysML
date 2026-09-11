package runtime

import (
	"fmt"
	"math"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
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
	if frame.IsScale() {
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
			ErrUnevaluableLibraryFunction, name)
	case placed == nil:
		return mapped, nil
	case mapped == nil:
		return placed, nil
	}
	mappedOrigin, err := semantics.ConvertQuantity(mapped.origin, placed.origin.Unit)
	if err != nil || !sameReference(placed.source, mapped.source) {
		return nil, fmt.Errorf("%w: %s: its transformation places zero on %s but its quantityValueMapping places it on %s",
			ErrUnevaluableLibraryFunction, name, placed.source.Name(), mapped.source.Name())
	}
	if !nearlyEqual(placed.origin.Num.AsReal(), mappedOrigin.Num.AsReal()) {
		return nil, fmt.Errorf("%w: %s: its transformation places zero at %s but its quantityValueMapping places it at %s",
			ErrUnevaluableLibraryFunction, name, placed.origin.String(), mappedOrigin.String())
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
			ErrUnevaluableLibraryFunction, name, t.Name(), t.shapeName())
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
			ErrUnevaluableLibraryFunction, name, t.Name(), describeValue(t.Placement.Origin), t.Source.Name())
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
	if vq := dir.VectorQuantity(); !ok && vq != nil && vq.Dimension() == 1 {
		if vq.Frame != nil && !vq.Frame.equal(t.Source) {
			return fmt.Errorf("%w: %s: the basisDirection of its transformation %s is a vector quantity in %s, not a quantity on %s, its source",
				ErrTypeMismatch, name, t.Name(), vq.Frame.Name(), t.Source.Name())
		}
		q, ok = vq.component(0), true
	}
	if !ok {
		return fmt.Errorf("%w: %s: a basisDirection of its transformation %s is %s, not a quantity on %s",
			ErrUnevaluableLibraryFunction, name, t.Name(), describeValue(dir), t.Source.Name())
	}
	onSource, err := semantics.ConvertQuantity(*q, t.Source.Axes[0])
	if err != nil {
		return fmt.Errorf("%w: %s: the basisDirection %s of its transformation %s is not on %s, its source: %w",
			ErrIncommensurableUnits, name, q.String(), t.Name(), t.Source.Name(), err)
	}
	if onSource.Num.AsReal() != 1 {
		return fmt.Errorf("%w: %s: the basisDirection %s of its transformation %s is not the identity 1 [%s], and the library gives a scale no other basis",
			ErrUnevaluableLibraryFunction, name, q.String(), t.Name(), t.Source.Name())
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
	num, err := semantics.MagnitudeArith(ast.OpSub, m.Reference.Num, mappedOnSource.Num)
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
	return !a.IsScale() && !b.IsScale() && len(a.Axes) == 1 && len(b.Axes) == 1 &&
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
		anchor, err := ctx.anchorOf(scale)
		if err != nil {
			return Quantity{}, err
		}
		onSource, err := semantics.ConvertQuantity(Quantity{Num: q.Num, Unit: scale.Scale.Unit}, differenceUnit(anchor.source))
		if err != nil {
			return Quantity{}, fmt.Errorf("%w: %s: its unit %s: %w", ErrIncommensurableUnits, ctx.scaleName(scale), scale.Scale.Unit, err)
		}
		num, err := semantics.MagnitudeArith(ast.OpAdd, onSource.Num, anchor.origin.Num)
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
	} else if err := ctx.checkCommensurable(q, scale.Scale.Unit); err != nil {
		return Quantity{}, err
	}
	anchor, err := ctx.anchorOf(scale)
	if err != nil {
		return Quantity{}, err
	}
	onSource, err := ctx.convertToReference(q, anchor.source)
	if err != nil {
		return Quantity{}, err
	}
	diff, err := semantics.MagnitudeArith(ast.OpSub, onSource.Num, anchor.origin.Num)
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
		from = scale.Scale.Unit
	}
	_, err := semantics.ConvertQuantity(Quantity{Num: q.Num, Unit: from}, unit)
	return err
}

// convertToReference is ConvertQuantity over a target that may be a scale, or a
// source that is: a quantity on a scale is carried to a ratio unit first.
func (ctx *Context) convertToReference(q Quantity, target *CoordinateFrame) (Quantity, error) {
	if target.IsScale() {
		return ctx.onScale(q, target)
	}
	if err := ctx.checkCommensurable(q, target.Axes[0]); err != nil {
		return Quantity{}, err
	}
	ratio, err := ctx.toRatioReference(q)
	if err != nil {
		return Quantity{}, err
	}
	return semantics.ConvertQuantity(ratio, target.Axes[0])
}

// conversionTarget reads ConvertQuantity's targetMRef: a unit or a scale, as the
// one-axis frame either is; a frame of more axes is no scalar reference.
func (ctx *Context) conversionTarget(name string, val Value) (*CoordinateFrame, error) {
	switch val.Kind {
	case ValMeasurementRef:
		return scalarFrame(val.MeasurementRef()), nil
	case ValCoordinateFrame:
		frame := val.CoordinateFrame()
		if frame.IsScale() {
			return frame, nil
		}
		return nil, fmt.Errorf("%w: function %s parameter %q requires a scalar measurement reference such as SI::m, got the coordinate frame %s of %d axes",
			ErrTypeMismatch, name, "targetMRef", frame.Name(), len(frame.Axes))
	}
	return nil, fmt.Errorf("%w: function %s parameter %q requires a measurement reference such as SI::m, got %s",
		ErrTypeMismatch, name, "targetMRef", describeValue(val))
}
