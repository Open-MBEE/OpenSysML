package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// Operators over points on a measurement scale (`26.85 [SI::'°C_abs']`): the affine
// cases ConvertQuantity's anchors define, and typed refusals of the rest.

const intervalScaleTypeFQN = "MeasurementReferences::IntervalScale"

// scaleKinds are the library's measurement scales, most specific first; a scale
// is described by the first it conforms to.
var scaleKinds = []struct{ fqn, name string }{
	{intervalScaleTypeFQN, "interval scale"},
	{"MeasurementReferences::OrdinalScale", "ordinal scale"},
	{"MeasurementReferences::CyclicRatioScale", "cyclic ratio scale"},
	{"MeasurementReferences::LogarithmicScale", "logarithmic scale"},
}

// pointOf is the measurement scale q is a point on; false for a magnitude in a unit.
func (ctx *Context) pointOf(q *Quantity) (*CoordinateFrame, bool, error) {
	if ctx == nil || ctx.model == nil || q == nil {
		return nil, false, nil
	}
	scale, ok, err := ctx.scaleOfUnit(q.Unit)
	if err != nil || !ok || scale == nil || !scale.IsScale() {
		return nil, false, err
	}
	return scale, true, nil
}

// isIntervalScale reports a scale whose points differ by magnitudes in its unit;
// an ordinal, cyclic ratio or logarithmic scale states no such structure.
func (ctx *Context) isIntervalScale(scale *CoordinateFrame) bool {
	return ctx.conformsToLibrary(scale.Type, intervalScaleTypeFQN)
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
	return fmt.Errorf("%w: %s on a point of %s: %s", ErrScalePoint, what, ctx.describeScale(scale), why)
}

// notIntervalError refuses what on a point of a scale that is no interval scale.
func (ctx *Context) notIntervalError(what string, scale *CoordinateFrame) error {
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

// operatorText names an operator in a refusal: `operator '*'`.
func operatorText(op ast.OperatorKind) string {
	return "operator '" + op.String() + "'"
}

// addQuantities is `+` or `-` over quantities: in the left unit for magnitudes;
// for points the affine cases point ± difference, difference + point, point − point.
func (ctx *Context) addQuantities(op ast.OperatorKind, left, right *Quantity) (Value, error) {
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
		return quantityResult(semantics.AddQuantities(op, *left, *right))
	case lpoint && rpoint:
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
	num, err := semantics.MagnitudeArith(op, point.Num, inUnit.Num)
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
	num, err := semantics.MagnitudeArith(ast.OpSub, left.Num, onLeft.Num)
	if err != nil {
		return Value{}, err
	}
	return quantityResult(semantics.InUnit(num, lscale.Scale.Unit))
}

// alignForComparison carries right onto left's reference so their magnitudes
// compare: onto the left scale, or through the right scale's anchor into the left unit.
func (ctx *Context) alignForComparison(left, right Quantity) (Quantity, Quantity, error) {
	lscale, lpoint, err := ctx.pointOf(&left)
	if err != nil {
		return Quantity{}, Quantity{}, err
	}
	rscale, rpoint, err := ctx.pointOf(&right)
	if err != nil {
		return Quantity{}, Quantity{}, err
	}
	switch {
	case !lpoint && !rpoint, lpoint && rpoint && lscale.equal(rscale):
		return left, right, nil
	case isBareZero(left) || isBareZero(right):
		// A bare zero adopts the other operand's reference, a scale as much as a unit.
		return left, right, nil
	case lpoint && !ctx.isIntervalScale(lscale):
		return Quantity{}, Quantity{}, ctx.notIntervalError("comparison with another reference", lscale)
	case rpoint && !ctx.isIntervalScale(rscale):
		return Quantity{}, Quantity{}, ctx.notIntervalError("comparison with another reference", rscale)
	case lpoint:
		onLeft, err := ctx.onScale(right, lscale)
		if err != nil {
			return Quantity{}, Quantity{}, err
		}
		return left, onLeft, nil
	default:
		inLeft, err := ctx.convertToReference(right, scalarFrame(measurementRefOf(left.Unit).MeasurementRef()))
		if err != nil {
			return Quantity{}, Quantity{}, err
		}
		return left, inLeft, nil
	}
}

// isBareZero reports a zero with no unit, which adopts the other operand's reference.
func isBareZero(q Quantity) bool {
	return q.Unit.None() && q.Num.IsNumeric() && q.Num.AsReal() == 0
}

// compareMagnitudes orders two quantities on the left one's reference.
func (ctx *Context) compareMagnitudes(left, right *Quantity) (int, error) {
	l, r, err := ctx.alignForComparison(*left, *right)
	if err != nil {
		return 0, err
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
	return Value{}, fmt.Errorf("%w: '%s' is not an ordering", ErrUnsupportedOperator, op)
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

// negateQuantity negates a quantity's magnitude, keeping its unit and the
// magnitude's kind; a point has no negative.
func (ctx *Context) negateQuantity(q *Quantity) (Value, error) {
	if err := ctx.refusePoints("negation", "a point has no negative; negate a difference between points instead", q); err != nil {
		return Value{}, err
	}
	return quantityResult(semantics.NegateQuantity(*q))
}
