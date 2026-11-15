package runtime

import (
	"fmt"
	"math"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Operators over points on a measurement scale (`26.85 [SI::'°C_abs']`): the affine
// cases ConvertQuantity's anchors define, and typed refusals of the rest.

const (
	intervalScaleTypeFQN    = "MeasurementReferences::IntervalScale"
	ordinalScaleTypeFQN     = "MeasurementReferences::OrdinalScale"
	cyclicRatioScaleTypeFQN = "MeasurementReferences::CyclicRatioScale"
	logarithmicScaleTypeFQN = "MeasurementReferences::LogarithmicScale"
)

// scaleKind is which of the library's measurement scales a scale is, 0 for none of them.
type scaleKind int

const (
	intervalScale scaleKind = iota + 1
	ordinalScale
	cyclicRatioScale
	logarithmicScale
)

// scaleKinds are the library's measurement scales, most specific first; a scale
// is described by the first it conforms to.
var scaleKinds = []struct {
	kind      scaleKind
	fqn, name string
}{
	{intervalScale, intervalScaleTypeFQN, "interval scale"},
	{ordinalScale, ordinalScaleTypeFQN, "ordinal scale"},
	{cyclicRatioScale, cyclicRatioScaleTypeFQN, "cyclic ratio scale"},
	{logarithmicScale, logarithmicScaleTypeFQN, "logarithmic scale"},
}

// scaleKindOf is the library scale a type's values are.
func (ctx *Context) scaleKindOf(typ *symbols.Symbol) scaleKind {
	for _, kind := range scaleKinds {
		if ctx.conformsToLibrary(typ, kind.fqn) {
			return kind.kind
		}
	}
	return 0
}

// pointOf is the measurement scale q is a point on; false for a magnitude in a unit.
func (ctx *Context) pointOf(q *Quantity) (*CoordinateFrame, bool, error) {
	if ctx == nil || ctx.model == nil || q == nil {
		return nil, false, nil
	}
	scale, ok, err := ctx.scaleOfUnit(q.Unit)
	if err != nil || !ok || scale == nil || !scale.isScale() {
		return nil, false, err
	}
	return scale, true, nil
}

// isIntervalScale reports a scale whose points differ by magnitudes in its unit.
func (ctx *Context) isIntervalScale(scale *CoordinateFrame) bool {
	return ctx.scaleKindOf(scale.Type) == intervalScale
}

// describeScale names a scale with its kind: `the interval scale SI::'°C_abs'`.
func (ctx *Context) describeScale(scale *CoordinateFrame) string {
	for _, kind := range scaleKinds {
		if ctx.conformsToLibrary(scale.Type, kind.fqn) {
			return "the " + kind.name + " " + ctx.scaleName(scale)
		}
	}
	return "the measurement scale " + ctx.scaleName(scale)
}

// pointError refuses what on a point of scale, saying why a point has no such thing.
func (ctx *Context) pointError(what string, scale *CoordinateFrame, why string) error {
	return fmt.Errorf("%w: %s on a point of %s: %s", errScalePoint, what, ctx.describeScale(scale), why)
}

// notIntervalError refuses what on a point of a scale that is no interval scale,
// saying what its kind does give its values.
func (ctx *Context) notIntervalError(what string, scale *CoordinateFrame) error {
	switch ctx.scaleKindOf(scale.Type) {
	case ordinalScale:
		return ctx.pointError(what, scale, "only the ordering of the values of an OrdinalScale is meaningful, not their intervals or ratios")
	case logarithmicScale:
		return ctx.pointError(what, scale, "a LogarithmicScale defines the value of a quantity, not an interval between values; convert to a unit with ConvertQuantity first")
	}
	return ctx.pointError(what, scale,
		"only an IntervalScale relates its points by differences in its unit")
}

// refusePoints refuses what when either operand is a point, naming the scale.
func (ctx *Context) refusePoints(what string, why string, operands ...*Quantity) error {
	for _, q := range operands {
		scale, ok, err := ctx.pointOf(q)
		if err != nil {
			return err
		}
		if ok {
			return ctx.pointError(what, scale, why)
		}
	}
	return nil
}

// cyclicScaleOf is the cyclic ratio scale q is a value on; nil for anything else.
func (ctx *Context) cyclicScaleOf(q *Quantity) (*CoordinateFrame, error) {
	scale, ok, err := ctx.pointOf(q)
	if err != nil || !ok || ctx.scaleKindOf(scale.Type) != cyclicRatioScale {
		return nil, err
	}
	return scale, nil
}

// ratioMagnitude is a value on a cyclic ratio scale as the magnitude in the scale's
// unit it is: a ratio scale's zero is its unit's, so a mapping may only confirm that.
func (ctx *Context) ratioMagnitude(q Quantity, scale *CoordinateFrame) (Quantity, error) {
	if scale.Scale.Mapping != nil {
		name := ctx.scaleName(scale)
		anchor, err := ctx.mappingAnchor(name, scale)
		if err != nil {
			return Quantity{}, err
		}
		if anchor.source.isScale() || anchor.origin.Num.AsReal() != 0 {
			return Quantity{}, fmt.Errorf("%w: %s: its quantityValueMapping places zero at %s on %s; a CyclicRatioScale is a ratio scale, whose zero is the zero of its unit %s",
				errUnevaluableLibraryFunction, name, anchor.origin.String(), anchor.source.Name(), scale.Scale.Unit)
		}
	}
	return Quantity{Num: q.Num, Unit: scale.Scale.Unit}, nil
}

// cyclicOperands are the operands with each value on a cyclic ratio scale entered as
// the magnitude it is; cyclic[i] is the scale operand i was on.
func (ctx *Context) cyclicOperands(operands ...*Quantity) ([]*Quantity, []*CoordinateFrame, error) {
	out, cyclic := make([]*Quantity, len(operands)), make([]*CoordinateFrame, len(operands))
	for i, q := range operands {
		scale, err := ctx.cyclicScaleOf(q)
		if err != nil {
			return nil, nil, err
		}
		out[i] = q
		if scale == nil {
			continue
		}
		magnitude, err := ctx.ratioMagnitude(*q, scale)
		if err != nil {
			return nil, nil, err
		}
		out[i], cyclic[i] = &magnitude, scale
	}
	return out, cyclic, nil
}

// ratioOperands is cyclicOperands for an operation only ratio magnitudes have:
// what is refused over a point on any other scale.
func (ctx *Context) ratioOperands(what string, why string, operands ...*Quantity) ([]*Quantity, []*CoordinateFrame, error) {
	out, cyclic, err := ctx.cyclicOperands(operands...)
	if err != nil {
		return nil, nil, err
	}
	return out, cyclic, ctx.refusePoints(what, why, out...)
}

// onCyclicScale is a magnitude as a value on the cyclic ratio scale: in the scale's unit, on the scale.
func (ctx *Context) onCyclicScale(val Value, scale *CoordinateFrame) (Value, error) {
	q, ok := asQuantity(val)
	if !ok {
		return val, nil
	}
	inUnit, err := semantics.ConvertQuantity(*q, scale.Scale.Unit)
	if err != nil {
		return Value{}, err
	}
	return NewQuantityValue(&Quantity{Num: inUnit.Num, Unit: scale.Axes[0]}), nil
}

// operatorText names an operator in a refusal: `operator '*'`.
func operatorText(op ast.OperatorKind) string {
	return "operator '" + op.String() + "'"
}

// addQuantities is `+` or `-` over quantities, on the left one's reference: a value on a
// cyclic ratio scale enters as the magnitude it is, and a result on that scale stays on it.
func (ctx *Context) addQuantities(op ast.OperatorKind, left, right *Quantity) (Value, error) {
	operands, cyclic, err := ctx.cyclicOperands(left, right)
	if err != nil {
		return Value{}, err
	}
	val, err := ctx.addOnReferences(op, operands[0], operands[1])
	if err != nil || cyclic[0] == nil {
		return val, err
	}
	if q, ok := asQuantity(val); ok {
		if _, point, err := ctx.pointOf(q); err != nil || point {
			return val, err
		}
	}
	return ctx.onCyclicScale(val, cyclic[0])
}

// addOnReferences is `+` or `-` in the left unit for magnitudes; for points the
// affine cases point ± difference, difference + point, point − point.
func (ctx *Context) addOnReferences(op ast.OperatorKind, left, right *Quantity) (Value, error) {
	lscale, lpoint, err := ctx.pointOf(left)
	if err != nil {
		return Value{}, err
	}
	rscale, rpoint, err := ctx.pointOf(right)
	if err != nil {
		return Value{}, err
	}
	switch {
	case !lpoint && !rpoint:
		return quantityResult(semantics.AddQuantities(op, *left, *right, ctx.maxIntegerBits))
	case lpoint && rpoint:
		if op == ast.OpAdd && !ctx.isIntervalScale(lscale) {
			return Value{}, ctx.notIntervalError(operatorText(op), lscale)
		}
		if op == ast.OpAdd {
			return Value{}, ctx.pointError(operatorText(op), lscale,
				fmt.Sprintf("two points have no sum; their difference `x - y` is a magnitude in %s, which a point adds", lscale.Scale.Unit))
		}
		return ctx.subtractPoints(left, lscale, right, rscale)
	case lpoint:
		return ctx.offsetPoint(op, left, lscale, right)
	default:
		if op == ast.OpSub {
			return Value{}, ctx.pointError("subtracting a point from a magnitude", rscale,
				fmt.Sprintf("a magnitude in %s minus a point is neither a point nor a magnitude; write `point - magnitude`", left.Unit))
		}
		return ctx.offsetPoint(op, right, rscale, left)
	}
}

// offsetPoint moves a point along its scale by a difference: the difference is
// expressed in the scale's unit and added to or taken from the point's magnitude.
func (ctx *Context) offsetPoint(op ast.OperatorKind, point *Quantity, scale *CoordinateFrame, diff *Quantity) (Value, error) {
	if !ctx.isIntervalScale(scale) {
		return Value{}, ctx.notIntervalError(operatorText(op), scale)
	}
	inUnit, err := semantics.ConvertQuantity(*diff, scale.Scale.Unit)
	if err != nil {
		return Value{}, fmt.Errorf("%w; a point on %s moves by a magnitude in its unit %s",
			err, ctx.describeScale(scale), scale.Scale.Unit)
	}
	num, err := semantics.MagnitudeArith(op, point.Num, inUnit.Num, ctx.maxIntegerBits)
	if err != nil {
		return Value{}, err
	}
	return NewQuantityValue(&Quantity{Num: num, Unit: point.Unit}), nil
}

// subtractPoints is the difference between two points as a magnitude in the left
// scale's unit; a point on another scale is carried onto the left scale first.
func (ctx *Context) subtractPoints(left *Quantity, lscale *CoordinateFrame, right *Quantity, rscale *CoordinateFrame) (Value, error) {
	if !ctx.isIntervalScale(lscale) {
		return Value{}, ctx.notIntervalError(operatorText(ast.OpSub), lscale)
	}
	if !ctx.isIntervalScale(rscale) {
		return Value{}, ctx.notIntervalError(operatorText(ast.OpSub), rscale)
	}
	onLeft, err := ctx.onScale(*right, lscale)
	if err != nil {
		return Value{}, err
	}
	num, err := semantics.MagnitudeArith(ast.OpSub, left.Num, onLeft.Num, ctx.maxIntegerBits)
	if err != nil {
		return Value{}, err
	}
	return quantityResult(semantics.InUnit(num, lscale.Scale.Unit))
}

// alignForComparison carries right onto left's reference so their magnitudes compare;
// cycle is the period, in their unit, cyclic values are equivalent over, 0 for none.
func (ctx *Context) alignForComparison(left, right Quantity) (l, r Quantity, cycle float64, err error) {
	lscale, lpoint, err := ctx.pointOf(&left)
	if err != nil {
		return Quantity{}, Quantity{}, 0, err
	}
	rscale, rpoint, err := ctx.pointOf(&right)
	if err != nil {
		return Quantity{}, Quantity{}, 0, err
	}
	switch {
	case !lpoint && !rpoint:
		return left, right, 0, nil
	case lpoint && rpoint && lscale.equal(rscale):
		return left, right, lscale.Scale.Modulus, nil
	case isBareZero(right) && lpoint:
		// A bare zero adopts the other operand's reference, a scale as much as a unit.
		return left, right, lscale.Scale.Modulus, nil
	case isBareZero(left) && rpoint:
		return left, right, rscale.Scale.Modulus, nil
	case isBareZero(left) || isBareZero(right):
		return left, right, 0, nil
	}
	const what = "comparison with another reference"
	left, lcycle, err := ctx.comparableOperand(what, left, lscale, true)
	if err != nil {
		return Quantity{}, Quantity{}, 0, err
	}
	right, rcycle, err := ctx.comparableOperand(what, right, rscale, false)
	if err != nil {
		return Quantity{}, Quantity{}, 0, err
	}
	unit := left.Unit
	if _, point, _ := ctx.pointOf(&left); point {
		if rcycle != nil && !ctx.isIntervalScale(lscale) {
			return Quantity{}, Quantity{}, 0, ctx.pointError(what, lscale, "a value on a cyclic ratio scale is equivalent over a cycle, which has no period on this scale; compare on the cyclic scale instead")
		}
		unit = lscale.Scale.Unit
		right, err = ctx.onScale(right, lscale)
	} else {
		right, err = ctx.convertToReference(right, unitFrame(left.Unit))
	}
	if err != nil {
		return Quantity{}, Quantity{}, 0, err
	}
	cycle, err = ctx.sharedCycle(unit, lcycle, rcycle)
	return left, right, cycle, err
}

// comparableOperand is an operand as two references compare it: a cyclic value as its magnitude and
// cycle, a logarithmic one on its scale when leading, else as its quantity; an ordinal one is refused.
func (ctx *Context) comparableOperand(what string, q Quantity, scale *CoordinateFrame, leading bool) (Quantity, *Quantity, error) {
	if scale == nil {
		return q, nil, nil
	}
	switch ctx.scaleKindOf(scale.Type) {
	case intervalScale:
		return q, nil, nil
	case cyclicRatioScale:
		magnitude, err := ctx.ratioMagnitude(q, scale)
		return magnitude, &Quantity{Num: semantics.Value{Kind: semantics.ValReal, Real: scale.Scale.Modulus}, Unit: scale.Scale.Unit}, err
	case logarithmicScale:
		if leading {
			return q, nil, nil
		}
		magnitude, err := ctx.fromLogarithm(q, scale)
		return magnitude, nil, err
	}
	return Quantity{}, nil, ctx.notIntervalError(what, scale)
}

// sharedCycle is the one period, in unit, the cyclic operands of a comparison are
// equivalent over; two scales of different periods state two equivalences, so they are refused.
func (ctx *Context) sharedCycle(unit Unit, cycles ...*Quantity) (float64, error) {
	var shared float64
	for _, cycle := range cycles {
		if cycle == nil {
			continue
		}
		inUnit, err := semantics.ConvertQuantity(*cycle, unit)
		if err != nil {
			return 0, err
		}
		period := inUnit.Num.AsReal()
		if shared != 0 && !nearlyEqual(shared, period) {
			return 0, fmt.Errorf("%w: comparison of values on cyclic ratio scales whose cycles differ, %g %s and %g %s: each states its own equivalence",
				errScalePoint, shared, unit, period, unit)
		}
		shared = period
	}
	return shared, nil
}

// principalValue is the representative in [0, cycle) of the values equivalent to x.
func principalValue(x, cycle float64) float64 {
	p := math.Mod(x, cycle)
	if p < 0 {
		p += cycle
	}
	if p >= cycle {
		return 0
	}
	return p
}

// isBareZero reports a zero with no unit, which adopts the other operand's reference.
func isBareZero(q Quantity) bool {
	return q.Unit.None() && q.Num.IsNumeric() && q.Num.AsReal() == 0
}

// compareMagnitudes orders two quantities on the left one's reference; values a
// cyclic ratio scale makes equivalent are equal, neither less nor greater.
func (ctx *Context) compareMagnitudes(left, right *Quantity) (int, error) {
	l, r, cycle, err := ctx.alignForComparison(*left, *right)
	if err != nil {
		return 0, err
	}
	if cycle != 0 && l.Num.IsNumeric() && r.Num.IsNumeric() {
		// Equivalent values differ by whole cycles; the difference is the one `-` computes.
		diff, err := semantics.MagnitudeArith(ast.OpSub, l.Num, r.Num, ctx.maxIntegerBits)
		if err == nil && math.Remainder(diff.AsReal(), cycle) == 0 {
			return 0, nil
		}
	}
	return semantics.CompareMagnitudes(l, r)
}

// compareQuantities is one of the four orderings over quantities, the right one
// carried onto the left one's reference.
func (ctx *Context) compareQuantities(op ast.OperatorKind, left, right *Quantity) (Value, error) {
	c, err := ctx.compareMagnitudes(left, right)
	if err != nil {
		return Value{}, err
	}
	switch op {
	case ast.OpLt:
		return boolValue(c < 0), nil
	case ast.OpLe:
		return boolValue(c <= 0), nil
	case ast.OpGt:
		return boolValue(c > 0), nil
	case ast.OpGe:
		return boolValue(c >= 0), nil
	}
	return Value{}, fmt.Errorf("%w: '%s' is not an ordering", errUnsupportedOperator, op)
}

// equalQuantities is `==` or `!=` over quantities on the left one's reference;
// incommensurable units are an error, not an inequality.
func (ctx *Context) equalQuantities(op ast.OperatorKind, left, right *Quantity) (Value, error) {
	c, err := ctx.compareMagnitudes(left, right)
	if err != nil {
		return Value{}, err
	}
	return boolValue((c == 0) != (op == ast.OpNeq)), nil
}

// negateQuantity negates a quantity's magnitude, keeping its unit and the magnitude's
// kind; a point has no negative, a value on a cyclic ratio scale has.
func (ctx *Context) negateQuantity(q *Quantity) (Value, error) {
	if _, _, err := ctx.ratioOperands("negation", "a point has no negative; negate a difference between points instead", q); err != nil {
		return Value{}, err
	}
	return quantityResult(semantics.NegateQuantity(*q))
}
