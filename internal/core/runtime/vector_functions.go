package runtime

import (
	"fmt"
	"math"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// ---------------------------------------------------------------------------
// VectorFunctions and the vector arithmetic VectorCalculations shares with it.
// ---------------------------------------------------------------------------

// vectorOperand is a vector argument as arithmetic reads it: its numbers, for a
// vector quantity the unit of each axis (nil for a plain vector), and the frame
// it was written over, if any.
type vectorOperand struct {
	num   []semantics.Value
	units []Unit
	frame *CoordinateFrame
}

func (v vectorOperand) dimension() int { return len(v.num) }

func (v vectorOperand) hasUnits() bool { return v.units != nil }

// axis is component i as a scalar quantity; a plain vector's is of dimension one.
func (v vectorOperand) axis(i int) *Quantity {
	if v.units == nil {
		return &Quantity{Num: v.num[i], Unit: semantics.UnitOne()}
	}
	return &Quantity{Num: v.num[i], Unit: v.units[i]}
}

// reals is the components as Reals, a vector quantity's converted into the unit
// of its first axis so that the magnitudes are commensurable.
func (v vectorOperand) reals(name string) ([]float64, error) {
	out := make([]float64, len(v.num))
	for i, n := range v.num {
		if !v.hasUnits() {
			out[i] = asReal(n)
			continue
		}
		converted, err := v.axis(i).ConvertTo(v.units[0])
		if err != nil {
			return nil, functionError(name, err)
		}
		out[i] = converted
	}
	return out, nil
}

// readVector reads a vector argument: a vector, a vector quantity, or a sequence
// of numbers, which VectorValues declares a NumericalVectorValue to be.
func readVector(name, label string, val Value) (vectorOperand, error) {
	switch val.Kind {
	case ValVector:
		return vectorOperand{num: val.Vector().Elements}, nil
	case ValVectorQuantity:
		vq := val.VectorQuantity()
		return vectorOperand{num: vq.Num, units: vq.Units, frame: vq.Frame}, nil
	}
	num, err := labelledVectorElements(name, label, val)
	if err != nil {
		return vectorOperand{}, err
	}
	return vectorOperand{num: num}, nil
}

// vectorElements is the numbers of a vector argument: a vector, a sequence of
// numbers, one number (the one-dimensional vector) or null (the empty vector).
func vectorElements(name, param string, val Value) ([]semantics.Value, error) {
	return labelledVectorElements(name, fmt.Sprintf("%q", param), val)
}

// labelledVectorElements is vectorElements reporting the parameter by a
// rendered label, which names an anonymous parameter by position.
func labelledVectorElements(name, label string, val Value) ([]semantics.Value, error) {
	switch val.Kind {
	case ValVector:
		return val.Vector().Elements, nil
	case ValVectorQuantity:
		return nil, fmt.Errorf(
			"%w: function %s parameter %s requires a vector of numbers, got %s",
			ErrTypeMismatch, name, label, describeValue(val),
		)
	}
	elements := elementsOf(val)
	out := make([]semantics.Value, len(elements))
	for i, elem := range elements {
		if elem.Kind != ValConst || !elem.Const.IsNumeric() {
			return nil, fmt.Errorf(
				"%w: function %s parameter %s requires a vector of numeric values, element %d is %s",
				ErrTypeMismatch, name, label, i+1, describeValue(elem),
			)
		}
		out[i] = elem.Const
	}
	return out, nil
}

// realElements is vectorElements widened to Real, for the CartesianVectorValue
// operations, whose elements the library declares Real.
func realElements(name, param string, val Value) ([]float64, error) {
	elements, err := vectorElements(name, param, val)
	if err != nil {
		return nil, err
	}
	out := make([]float64, len(elements))
	for i, elem := range elements {
		out[i] = asReal(elem)
	}
	return out, nil
}

// vectorValue builds a vector from its elements, charging them against the run's
// element budget; an element outside the range of its kind is reported.
func (ctx *Context) vectorValue(elements []semantics.Value) (Value, error) {
	checked, err := ctx.checkedComponents(elements)
	if err != nil {
		return Value{}, err
	}
	return NewVectorValue(checked), nil
}

// vectorQuantityValue builds a vector quantity of the components, each in the
// unit at the same axis; a quantity's num is Number[1..*], so none is a violation.
func (ctx *Context) vectorQuantityValue(num []semantics.Value, units []Unit) (Value, error) {
	if len(num) == 0 {
		return Value{}, errEmptyVectorQuantity()
	}
	checked, err := ctx.checkedComponents(num)
	if err != nil {
		return Value{}, err
	}
	return NewVectorQuantityValue(checked, units), nil
}

// errEmptyVectorQuantity rejects the vector quantity of no components.
func errEmptyVectorQuantity() error {
	return fmt.Errorf(
		"%w: %s: a vector quantity of no components, its num is Number[1..*]",
		ErrMultiplicityViolation, vectorQuantityTypeFQN,
	)
}

// checkedComponents charges and screens the components of a vector being built.
func (ctx *Context) checkedComponents(elements []semantics.Value) ([]semantics.Value, error) {
	if err := ctx.chargeElements(int64(len(elements))); err != nil {
		return nil, err
	}
	out := make([]semantics.Value, len(elements))
	for i, elem := range elements {
		checked, err := checkedNumeric(elem)
		if err != nil {
			return nil, err
		}
		out[i] = checked
	}
	return out, nil
}

// vectorOperandValue is the vector or vector quantity an operand was read from.
func (ctx *Context) vectorOperandValue(v vectorOperand) (Value, error) {
	if v.frame != nil {
		return ctx.framedVectorQuantity(v.num, v.frame)
	}
	if v.hasUnits() {
		return ctx.vectorQuantityValue(v.num, v.units)
	}
	return ctx.vectorValue(v.num)
}

// framedResult is vectorResult over a frame, each axis in the frame's unit for it;
// over no frame it is vectorResult.
func (ctx *Context) framedResult(name string, frame *CoordinateFrame, axes []Value) (Value, error) {
	if frame == nil {
		return ctx.vectorResult(name, axes)
	}
	num := make([]semantics.Value, len(axes))
	for i, axis := range axes {
		q, ok := asQuantity(axis)
		if !ok {
			return Value{}, fmt.Errorf("%w: function %s computed %s for a component",
				ErrTypeMismatch, name, describeValue(axis))
		}
		converted, err := semantics.ConvertQuantity(*q, frame.Axes[i])
		if err != nil {
			return Value{}, functionError(name, err)
		}
		num[i] = converted.Num
	}
	return ctx.framedVectorQuantity(num, frame)
}

// sharedFrame is the frame two operands of one operation are over: the same frame,
// or none when neither is over any; a vector over a frame and one over none, or
// vectors over two frames, have no common coordinates to combine.
func sharedFrame(name string, v, w vectorOperand) (*CoordinateFrame, error) {
	switch {
	case v.frame == nil && w.frame == nil:
		return nil, nil
	case v.frame != nil && w.frame != nil && v.frame.equal(w.frame):
		return v.frame, nil
	case v.frame != nil && w.frame != nil:
		return nil, fmt.Errorf("%w: function %s combines vectors over the coordinate frames %s and %s; transform one into the other's frame first",
			ErrTypeMismatch, name, v.frame.Name(), w.frame.Name())
	}
	framed := v.frame
	if framed == nil {
		framed = w.frame
	}
	return nil, fmt.Errorf("%w: function %s combines a vector over the coordinate frame %s with one over no frame; write both over the frame (`(x, y, z) [%s]`)",
		ErrTypeMismatch, name, framed.Name(), framed.Name())
}

// scaledFrame is the frame a vector over frame is over once scaled by x: the frame
// itself for a number, else the frame composed with x's unit (`CoordinateFrame*`, `/`).
func (ctx *Context) scaledFrame(name string, op ast.OperatorKind, frame *CoordinateFrame, x *Quantity) (*CoordinateFrame, error) {
	if frame == nil || x.Unit.None() {
		return frame, nil
	}
	composed, _, err := ctx.composeFrame(op, NewCoordinateFrameValue(frame), measurementRefOf(x.Unit))
	if err != nil {
		return nil, functionError(name, err)
	}
	return composed.CoordinateFrame(), nil
}

// vectorResult is the vector of per-axis scalar results: a vector quantity when
// an axis carries a unit, else a plain vector.
func (ctx *Context) vectorResult(name string, axes []Value) (Value, error) {
	num := make([]semantics.Value, len(axes))
	units := make([]Unit, len(axes))
	withUnit := false
	for i, axis := range axes {
		q, ok := asQuantity(axis)
		if !ok {
			return Value{}, fmt.Errorf("%w: function %s computed %s for a component",
				ErrTypeMismatch, name, describeValue(axis))
		}
		num[i], units[i] = q.Num, q.Unit
		withUnit = withUnit || axis.Kind == ValQuantity
	}
	if !withUnit {
		return ctx.vectorValue(num)
	}
	return ctx.vectorQuantityValue(num, units)
}

// checkedNumeric screens a computed numeric value, so an arithmetic result that
// overflowed the Real range is reported rather than returned as an infinity.
func checkedNumeric(v semantics.Value) (semantics.Value, error) {
	if v.Kind != semantics.ValReal {
		return v, nil
	}
	return semantics.RealResult(v.Real)
}

// isNumeric reports whether a value is an Integer or a Real.
func isNumeric(v semantics.Value) bool {
	return v.Kind == semantics.ValInt || v.Kind == semantics.ValReal
}

// elementArith applies an arithmetic operator to two numeric elements, reporting
// a result outside the range of its kind rather than wrapping or infinite.
func elementArith(name string, op ast.OperatorKind, a, b semantics.Value) (semantics.Value, error) {
	if a.Kind == semantics.ValInt && b.Kind == semantics.ValInt {
		result, ok := semantics.IntArith(op, a.Int, b.Int)
		if !ok {
			return semantics.Value{}, fmt.Errorf(
				"%w: function %s has a result outside the Integer range",
				semantics.ErrArithmeticOverflow, name,
			)
		}
		return semantics.Value{Kind: semantics.ValInt, Int: result}, nil
	}
	if a.IsNumeric() && b.IsNumeric() {
		res, ok := semantics.RealArith(op, toReal(a), toReal(b))
		if !ok {
			return semantics.Value{}, fmt.Errorf(
				"%w: function %s cannot combine its arguments", ErrTypeMismatch, name,
			)
		}
		return semantics.RealResult(res)
	}
	res, ok := semantics.EvalBinary(op, a, b)
	if !ok {
		// A sum, difference or product of two numbers is declined only for leaving
		// the Real range; anything else is an operator the arguments do not define.
		switch op {
		case ast.OpAdd, ast.OpSub, ast.OpMul:
			if isNumeric(a) && isNumeric(b) {
				return semantics.Value{}, fmt.Errorf(
					"%w: function %s has a result outside the Real range",
					semantics.ErrArithmeticOverflow, name,
				)
			}
		}
		return semantics.Value{}, fmt.Errorf(
			"%w: function %s cannot combine its arguments", ErrTypeMismatch, name,
		)
	}
	return checkedNumeric(res)
}

// realVector builds a vector of Reals, reporting an element that is not a finite
// Real rather than carrying an infinity into it.
func (ctx *Context) realVector(components []float64) (Value, error) {
	elements := make([]semantics.Value, len(components))
	for i, x := range components {
		elem, err := semantics.RealResult(x)
		if err != nil {
			return Value{}, err
		}
		elements[i] = elem
	}
	return ctx.vectorValue(elements)
}

// checkedReal wraps a computed Real as a runtime value, reporting a result that
// is not a finite number.
func checkedReal(x float64) (Value, error) {
	res, err := semantics.RealResult(x)
	if err != nil {
		return Value{}, err
	}
	return Value{Kind: ValConst, Const: res}, nil
}

// argumentOmitted reports an argument as not given for a [0..1] parameter: null,
// and equally an empty collection, which KerML holds to be the same no value.
func argumentOmitted(val Value) bool {
	return len(elementsOf(val)) == 0
}

// scalarArg reads a scalar numeric argument: the NumericalValue a scalar-vector
// product or a Complex component is declared as.
func scalarArg(name, param string, val Value) (semantics.Value, error) {
	if val.Kind != ValConst || !val.Const.IsNumeric() {
		return semantics.Value{}, fmt.Errorf(
			"%w: function %s parameter %q requires a numeric value",
			ErrTypeMismatch, name, param,
		)
	}
	return val.Const, nil
}

// dimensionMismatch is the error for two vectors an operation needs of equal dimension.
func dimensionMismatch(name string, v, w vectorOperand) error {
	return fmt.Errorf(
		"%w: function %s requires vectors of equal dimension, got %d and %d",
		ErrTypeMismatch, name, v.dimension(), w.dimension(),
	)
}

// combineElements applies an arithmetic operator elementwise to two vectors of
// equal dimension, keeping the elements' kind as the library's declaration over
// NumericalValue does: two Integer vectors give an Integer vector.
func combineElements(name string, op ast.OperatorKind, v, w []semantics.Value) ([]semantics.Value, error) {
	if len(v) != len(w) {
		return nil, dimensionMismatch(name, vectorOperand{num: v}, vectorOperand{num: w})
	}
	out := make([]semantics.Value, len(v))
	for i := range v {
		res, err := elementArith(name, op, v[i], w[i])
		if err != nil {
			return nil, err
		}
		out[i] = res
	}
	return out, nil
}

// combineVectors is the elementwise sum or difference of two vectors; with a
// vector quantity involved each axis is added as the scalar quantities are.
func (ctx *Context) combineVectors(name string, op ast.OperatorKind, v, w vectorOperand) (Value, error) {
	if !v.hasUnits() && !w.hasUnits() {
		out, err := combineElements(name, op, v.num, w.num)
		if err != nil {
			return Value{}, err
		}
		return ctx.vectorValue(out)
	}
	if v.dimension() != w.dimension() {
		return Value{}, dimensionMismatch(name, v, w)
	}
	frame, err := sharedFrame(name, v, w)
	if err != nil {
		return Value{}, err
	}
	axes := make([]Value, v.dimension())
	for i := range axes {
		axis, err := ctx.addQuantities(op, v.axis(i), w.axis(i))
		if err != nil {
			return Value{}, functionError(name, err)
		}
		axes[i] = axis
	}
	return ctx.framedResult(name, frame, axes)
}

// zeroLike is the zero of a numeric value's kind, which negation subtracts from.
func zeroLike(v semantics.Value) semantics.Value {
	if v.Kind == semantics.ValInt {
		return semantics.Value{Kind: semantics.ValInt}
	}
	return semantics.Value{Kind: semantics.ValReal}
}

// vectorIsZero is isZeroVector and its Cartesian specialization: a zero vector
// is one whose every element is zero.
func vectorIsZero(name string, _ *Context, args []Value) (Value, error) {
	v, err := readVector(name, `"v"`, args[0])
	if err != nil {
		return Value{}, err
	}
	for _, elem := range v.num {
		if asReal(elem) != 0 {
			return boolValue(false), nil
		}
	}
	return boolValue(true), nil
}

// vectorAdd is VectorFunctions::'+' and 'cartesian+': the sum of two vectors of
// equal dimension, or, given one argument, that vector.
func vectorAdd(name string, ctx *Context, args []Value) (Value, error) {
	v, err := readVector(name, `"v"`, args[0])
	if err != nil {
		return Value{}, err
	}
	if argumentOmitted(args[1]) {
		return ctx.vectorOperandValue(v)
	}
	w, err := readVector(name, `"w"`, args[1])
	if err != nil {
		return Value{}, err
	}
	return ctx.combineVectors(name, ast.OpAdd, v, w)
}

// vectorSubtract is VectorFunctions::'-' and 'cartesian-': the difference of two
// vectors of equal dimension, or, given one argument, the vector that added to it
// gives the zero vector.
func vectorSubtract(name string, ctx *Context, args []Value) (Value, error) {
	v, err := readVector(name, `"v"`, args[0])
	if err != nil {
		return Value{}, err
	}
	if argumentOmitted(args[1]) {
		return ctx.negateVector(name, v)
	}
	w, err := readVector(name, `"w"`, args[1])
	if err != nil {
		return Value{}, err
	}
	return ctx.combineVectors(name, ast.OpSub, v, w)
}

// negateVector is the vector that added to v gives the zero vector.
func (ctx *Context) negateVector(name string, v vectorOperand) (Value, error) {
	if !v.hasUnits() {
		zeros := make([]semantics.Value, v.dimension())
		for i, elem := range v.num {
			zeros[i] = zeroLike(elem)
		}
		negated, err := combineElements(name, ast.OpSub, zeros, v.num)
		if err != nil {
			return Value{}, err
		}
		return ctx.vectorValue(negated)
	}
	axes := make([]Value, v.dimension())
	for i := range axes {
		axis, err := ctx.negateQuantity(v.axis(i))
		if err != nil {
			return Value{}, functionError(name, err)
		}
		axes[i] = axis
	}
	return ctx.framedResult(name, v.frame, axes)
}

// vectorOf is VectorFunctions::VectorOf, the NumericalVectorValue of a non-empty
// list of components, whose kind it keeps.
func vectorOf(name string, ctx *Context, args []Value) (Value, error) {
	components, err := vectorElements(name, "components", args[0])
	if err != nil {
		return Value{}, err
	}
	if len(components) == 0 {
		return Value{}, fmt.Errorf(
			"%w: function %s requires at least one component (components: NumericalValue[1..*])",
			ErrMultiplicityViolation, name,
		)
	}
	return ctx.vectorValue(components)
}

// cartesianVectorOf is VectorFunctions::CartesianVectorOf, whose components the
// library declares Real, so an Integer component widens.
func cartesianVectorOf(name string, ctx *Context, args []Value) (Value, error) {
	components, err := realElements(name, "components", args[0])
	if err != nil {
		return Value{}, err
	}
	return ctx.realVector(components)
}

// cartesianThreeVectorOf is VectorFunctions::CartesianThreeVectorOf, which
// declares its components Real[3].
func cartesianThreeVectorOf(name string, ctx *Context, args []Value) (Value, error) {
	components, err := realElements(name, "components", args[0])
	if err != nil {
		return Value{}, err
	}
	if len(components) != 3 {
		return Value{}, fmt.Errorf(
			"%w: function %s requires 3 components (components: Real[3]), got %d",
			ErrMultiplicityViolation, name, len(components),
		)
	}
	return ctx.realVector(components)
}

// scalarVectorMult is the scalar product with the scalar first, which the library
// also aliases as VectorFunctions::'*'.
func scalarVectorMult(name string, ctx *Context, args []Value) (Value, error) {
	return scaleVector(name, ctx, args[0], "x", args[1], "v")
}

// vectorScalarMult is the same product with the vector first, which the library
// defines as scalarVectorMult(x, v).
func vectorScalarMult(name string, ctx *Context, args []Value) (Value, error) {
	return scaleVector(name, ctx, args[1], "x", args[0], "v")
}

// scaleVector multiplies every element of a vector by a number, keeping the
// elements' kind; a vector quantity keeps its units.
func scaleVector(name string, ctx *Context, scalar Value, scalarParam string, vector Value, vectorParam string) (Value, error) {
	x, err := scalarArg(name, scalarParam, scalar)
	if err != nil {
		return Value{}, err
	}
	v, err := readVector(name, fmt.Sprintf("%q", vectorParam), vector)
	if err != nil {
		return Value{}, err
	}
	if v.hasUnits() {
		return ctx.scaleVectorQuantity(name, ast.OpMul, &Quantity{Num: x, Unit: semantics.UnitOne()}, v)
	}
	scaled := make([]semantics.Value, v.dimension())
	for i, elem := range v.num {
		res, err := elementArith(name, ast.OpMul, x, elem)
		if err != nil {
			return Value{}, err
		}
		scaled[i] = res
	}
	return ctx.vectorValue(scaled)
}

// scaleVectorQuantity multiplies or divides each axis of a vector by a scalar
// quantity, composing the units as the scalar QuantityCalculations do.
func (ctx *Context) scaleVectorQuantity(name string, op ast.OperatorKind, x *Quantity, v vectorOperand) (Value, error) {
	if v.dimension() == 0 && !x.Unit.None() {
		return Value{}, functionError(name, errEmptyVectorQuantity())
	}
	frame, err := ctx.scaledFrame(name, op, v.frame, x)
	if err != nil {
		return Value{}, err
	}
	axes := make([]Value, v.dimension())
	for i := range axes {
		left, right := x, v.axis(i)
		if op == ast.OpDiv {
			left, right = v.axis(i), x
		}
		axis, err := ctx.scaleQuantities(op, left, right)
		if err != nil {
			return Value{}, functionError(name, err)
		}
		axes[i] = axis
	}
	return ctx.framedResult(name, frame, axes)
}

// scalarQuantityVectorMult is VectorCalculations::scalarQuantityVectorMult, a
// vector quantity scaled by a scalar quantity: `2 [m] * ⟨1, 2⟩` is `⟨2, 4⟩ [m]`.
func scalarQuantityVectorMult(name string, ctx *Context, args []Value) (Value, error) {
	return scaleVectorByQuantity(name, ctx, args[0], args[1])
}

// vectorScalarQuantityMult is the same product with the vector first.
func vectorScalarQuantityMult(name string, ctx *Context, args []Value) (Value, error) {
	return scaleVectorByQuantity(name, ctx, args[1], args[0])
}

// scaleVectorByQuantity multiplies a vector by a scalar quantity; both parameters
// are anonymous in the library, so they are reported by position.
func scaleVectorByQuantity(name string, ctx *Context, scalar, vector Value) (Value, error) {
	x, err := quantityArg(name, positionLabel(0), scalar)
	if err != nil {
		return Value{}, err
	}
	v, err := readVector(name, positionLabel(1), vector)
	if err != nil {
		return Value{}, err
	}
	return ctx.scaleVectorQuantity(name, ast.OpMul, x, v)
}

// vectorScalarDiv is VectorFunctions::vectorScalarDiv. The library defines it as
// scalarVectorMult(1.0 / x, v); dividing each element is the same quotient
// without the reciprocal's rounding.
func vectorScalarDiv(name string, ctx *Context, args []Value) (Value, error) {
	v, err := readVector(name, `"v"`, args[0])
	if err != nil {
		return Value{}, err
	}
	x, err := scalarArg(name, "x", args[1])
	if err != nil {
		return Value{}, err
	}
	if asReal(x) == 0 {
		return Value{}, fmt.Errorf("%w: function %s divides by zero", ErrDivisionByZero, name)
	}
	if v.hasUnits() {
		return ctx.scaleVectorQuantity(name, ast.OpDiv, &Quantity{Num: x, Unit: semantics.UnitOne()}, v)
	}
	quotients := make([]float64, v.dimension())
	for i, elem := range v.num {
		quotients[i] = asReal(elem) / asReal(x)
	}
	return ctx.realVector(quotients)
}

// vectorScalarQuantityDiv is VectorCalculations::vectorScalarQuantityDiv: each
// axis divided by a scalar quantity, `⟨2, 4⟩ [m] / 2 [s]` giving `⟨1.0, 2.0⟩ [m/s]`.
func vectorScalarQuantityDiv(name string, ctx *Context, args []Value) (Value, error) {
	v, err := readVector(name, positionLabel(0), args[0])
	if err != nil {
		return Value{}, err
	}
	x, err := quantityArg(name, positionLabel(1), args[1])
	if err != nil {
		return Value{}, err
	}
	if toReal(x.Num) == 0 {
		return Value{}, fmt.Errorf("%w: function %s divides by zero", ErrDivisionByZero, name)
	}
	return ctx.scaleVectorQuantity(name, ast.OpDiv, x, v)
}

// vectorInner is the inner product of two vectors of equal dimension, keeping the
// elements' kind: two Integer vectors have an Integer inner product. Vector
// quantities give the magnitude the library declares (Number), in the product of
// their first axes' units.
func vectorInner(name string, ctx *Context, args []Value) (Value, error) {
	v, err := readVector(name, `"v"`, args[0])
	if err != nil {
		return Value{}, err
	}
	w, err := readVector(name, `"w"`, args[1])
	if err != nil {
		return Value{}, err
	}
	if v.hasUnits() || w.hasUnits() {
		return ctx.innerQuantity(name, v, w)
	}
	products, err := combineElements(name, ast.OpMul, v.num, w.num)
	if err != nil {
		return Value{}, err
	}
	sum := semantics.Value{Kind: semantics.ValInt}
	for _, product := range products {
		next, err := elementArith(name, ast.OpAdd, sum, product)
		if err != nil {
			return Value{}, err
		}
		sum = next
	}
	checked, err := checkedNumeric(sum)
	if err != nil {
		return Value{}, err
	}
	return Value{Kind: ValConst, Const: checked}, nil
}

// innerQuantity is the inner product over quantity axes: the axis products summed
// in the unit the first composes, and that sum's magnitude, as the library declares
// the result a Number.
func (ctx *Context) innerQuantity(name string, v, w vectorOperand) (Value, error) {
	if v.dimension() != w.dimension() {
		return Value{}, dimensionMismatch(name, v, w)
	}
	if _, err := sharedFrame(name, v, w); err != nil {
		return Value{}, err
	}
	if v.dimension() == 0 {
		return integerValue(0), nil
	}
	var sum *Quantity
	for i := 0; i < v.dimension(); i++ {
		// The unit is dropped, so the products stay in the unit the axes compose.
		if err := ctx.refusePoints(name, scaleNotAFactor, v.axis(i), w.axis(i)); err != nil {
			return Value{}, functionError(name, err)
		}
		product, err := quantityResult(semantics.ScaleQuantities(ast.OpMul, *v.axis(i), *w.axis(i)))
		if err != nil {
			return Value{}, functionError(name, err)
		}
		term, _ := asQuantity(product)
		if i == 0 {
			sum = term
			continue
		}
		added, err := ctx.addQuantities(ast.OpAdd, sum, term)
		if err != nil {
			return Value{}, functionError(name, err)
		}
		sum, _ = asQuantity(added)
	}
	return Value{Kind: ValConst, Const: sum.Num}, nil
}

// refusePointAxes refuses what over vectors whose axes are points on a scale:
// such coordinates have no length or direction, only differences between them.
func (ctx *Context) refusePointAxes(name, what string, vectors ...vectorOperand) error {
	for _, v := range vectors {
		if !v.hasUnits() {
			continue
		}
		for i := 0; i < v.dimension(); i++ {
			if err := ctx.refusePoints(name, what+" of a vector of points is undefined; take differences between points first", v.axis(i)); err != nil {
				return functionError(name, err)
			}
		}
	}
	return nil
}

// vectorNorm is the norm (magnitude) of a vector, the square root of its inner
// product with itself; the library declares it a Number, so a vector quantity's is
// the magnitude in the unit of its first axis.
func vectorNorm(name string, ctx *Context, args []Value) (Value, error) {
	v, err := readVector(name, `"v"`, args[0])
	if err != nil {
		return Value{}, err
	}
	if err := ctx.refusePointAxes(name, "a norm", v); err != nil {
		return Value{}, err
	}
	reals, err := v.reals(name)
	if err != nil {
		return Value{}, err
	}
	return checkedReal(euclideanNorm(reals))
}

// euclideanNorm is the square root of the sum of the squares, accumulated so
// that a square outside the Real range does not overflow a finite norm.
func euclideanNorm(elements []float64) float64 {
	norm := 0.0
	for _, x := range elements {
		norm = math.Hypot(norm, x)
	}
	return norm
}

// vectorAngle is the angle between two vectors of equal dimension,
// arccos(inner(v, w) / (norm(v) * norm(w))). A zero vector points nowhere, so
// there is no angle to it. The ratio cancels any units, so a vector quantity's
// angle is that of its magnitudes.
func vectorAngle(name string, ctx *Context, args []Value) (Value, error) {
	vOperand, err := readVector(name, `"v"`, args[0])
	if err != nil {
		return Value{}, err
	}
	wOperand, err := readVector(name, `"w"`, args[1])
	if err != nil {
		return Value{}, err
	}
	if err := ctx.refusePointAxes(name, "an angle", vOperand, wOperand); err != nil {
		return Value{}, err
	}
	if vOperand.dimension() != wOperand.dimension() {
		return Value{}, dimensionMismatch(name, vOperand, wOperand)
	}
	if _, err := sharedFrame(name, vOperand, wOperand); err != nil {
		return Value{}, err
	}
	v, err := vOperand.reals(name)
	if err != nil {
		return Value{}, err
	}
	w, err := wOperand.reals(name)
	if err != nil {
		return Value{}, err
	}
	unitV, unitW := unitVector(v), unitVector(w)
	if unitV == nil || unitW == nil {
		return Value{}, fmt.Errorf(
			"%w: function %s has no angle to a zero vector",
			semantics.ErrArithmeticDomain, name,
		)
	}
	// The cosine is the inner product of the unit vectors, so components whose
	// product would leave the Real range still give a finite cosine.
	cosine := 0.0
	for i := range unitV {
		cosine += unitV[i] * unitW[i]
	}
	// Rounding can carry the cosine of two parallel vectors just outside
	// [-1.0, 1.0], where the arc cosine has no value; the angle there is 0 or pi.
	cosine = math.Max(-1, math.Min(1, cosine))
	return checkedReal(math.Acos(cosine))
}

// unitVector is the direction of a vector, nil for the zero vector. It is scaled
// by its largest component first, so a vector whose norm overflows still has one.
func unitVector(v []float64) []float64 {
	scale := 0.0
	for _, x := range v {
		scale = math.Max(scale, math.Abs(x))
	}
	if scale == 0 {
		return nil
	}
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = x / scale
	}
	norm := euclideanNorm(out)
	for i := range out {
		out[i] /= norm
	}
	return out
}

// isVectorKind reports whether a value is a vector or a vector quantity.
func isVectorKind(val Value) bool {
	return val.Kind == ValVector || val.Kind == ValVectorQuantity
}

// vectorArithmetic is operator notation over a vector operand, bound to the
// VectorFunctions or VectorCalculations operator the operand kinds select.
func (ctx *Context) vectorArithmetic(op ast.OperatorKind, left, right Value) (Value, bool, error) {
	lv, rv := isVectorKind(left), isVectorKind(right)
	if !lv && !rv {
		return Value{}, false, nil
	}
	args := []Value{left, right}
	switch op {
	case ast.OpAdd:
		if lv && rv {
			val, err := vectorAdd("VectorFunctions::'+'", ctx, args)
			return val, true, err
		}
	case ast.OpSub:
		if lv && rv {
			val, err := vectorSubtract("VectorFunctions::'-'", ctx, args)
			return val, true, err
		}
	case ast.OpMul:
		switch {
		case lv && rv:
			val, err := vectorInner("VectorFunctions::inner", ctx, args)
			return val, true, err
		case lv && right.Kind == ValQuantity:
			val, err := vectorScalarQuantityMult("VectorCalculations::vectorScalarQuantityMult", ctx, args)
			return val, true, err
		case lv && right.Kind == ValConst:
			val, err := vectorScalarMult("VectorFunctions::vectorScalarMult", ctx, args)
			return val, true, err
		case rv && left.Kind == ValQuantity:
			val, err := scalarQuantityVectorMult("VectorCalculations::scalarQuantityVectorMult", ctx, args)
			return val, true, err
		case rv && left.Kind == ValConst:
			val, err := scalarVectorMult("VectorFunctions::scalarVectorMult", ctx, args)
			return val, true, err
		}
	case ast.OpDiv:
		switch {
		case lv && right.Kind == ValQuantity:
			val, err := vectorScalarQuantityDiv("VectorCalculations::vectorScalarQuantityDiv", ctx, args)
			return val, true, err
		case lv && right.Kind == ValConst:
			val, err := vectorScalarDiv("VectorFunctions::vectorScalarDiv", ctx, args)
			return val, true, err
		}
	}
	return Value{}, false, nil
}

// vectorCollection reads a collection of vectors. A sequence literal flattens,
// so each element must be a vector value: a bare number is none.
func vectorCollection(name, param string, val Value) ([]vectorOperand, error) {
	elements := elementsOf(val)
	out := make([]vectorOperand, len(elements))
	for i, elem := range elements {
		if elem.Kind != ValVector && elem.Kind != ValVectorQuantity {
			return nil, fmt.Errorf(
				"%w: function %s requires a collection of vectors (%s: VectorValue[*]), element %d is %s; build each with VectorOf",
				ErrTypeMismatch, name, param, i+1, describeValue(elem),
			)
		}
		v, err := readVector(name, fmt.Sprintf("%q element %d", param, i+1), elem)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

// vectorSum0 is VectorFunctions::sum0: `coll->reduce '+' ?? zero`, the sum of a
// collection of vectors, or the given zero vector when it is empty.
func vectorSum0(name string, ctx *Context, args []Value) (Value, error) {
	coll, err := vectorCollection(name, "coll", args[0])
	if err != nil {
		return Value{}, err
	}
	zero, err := readVector(name, `"zero"`, args[1])
	if err != nil {
		return Value{}, err
	}
	for _, elem := range zero.num {
		if asReal(elem) != 0 {
			return Value{}, fmt.Errorf(
				"%w: function %s requires a zero vector (inv precondition { isZeroVector(zero) }), got %s",
				ErrTypeMismatch, name, FormatValue(args[1]),
			)
		}
	}
	if len(coll) == 0 {
		return ctx.vectorOperandValue(zero)
	}
	return ctx.sumVectors(name, coll)
}

// vectorSum is VectorFunctions::sum: sum0 over the collection with
// cartesian3DZeroVector as its zero, so each vector is a Cartesian three-vector.
func vectorSum(name string, ctx *Context, args []Value) (Value, error) {
	coll, err := vectorCollection(name, "coll", args[0])
	if err != nil {
		return Value{}, err
	}
	for i, v := range coll {
		if v.dimension() != 3 {
			return Value{}, fmt.Errorf(
				"%w: function %s requires Cartesian three-vectors (coll: CartesianThreeVectorValue[*]), element %d has dimension %d",
				ErrTypeMismatch, name, i+1, v.dimension(),
			)
		}
	}
	if len(coll) == 0 {
		return ctx.realVector([]float64{0, 0, 0})
	}
	return ctx.sumVectors(name, coll)
}

// sumVectors folds a non-empty collection of vectors with '+'.
func (ctx *Context) sumVectors(name string, coll []vectorOperand) (Value, error) {
	acc, err := ctx.vectorOperandValue(coll[0])
	if err != nil {
		return Value{}, err
	}
	for _, v := range coll[1:] {
		left, err := readVector(name, `"coll"`, acc)
		if err != nil {
			return Value{}, err
		}
		if acc, err = ctx.combineVectors(name, ast.OpAdd, left, v); err != nil {
			return Value{}, err
		}
	}
	return acc, nil
}
