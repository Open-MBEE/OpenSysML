package runtime

import (
	"fmt"
	"math"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// init registers the Quantities and Units domain library's calculation packages:
// computed over the quantity or vector representation, or unevaluable by name.
func init() {
	registerQuantityCalculations()
	registerMeasurementRefCalculations()
	registerVectorCalculations()
	registerTensorCalculations()
}

// registerQuantityCalculations registers QuantityCalculations over the quantity
// representation; `sum` folds in the first element's unit, not the body's unvalued `zero`.
func registerQuantityCalculations() {
	registerValueFunction("QuantityCalculations::[", []string{"num", "mRef"}, 2, quantityOf)
	registerValueFunction("QuantityCalculations::isZero", []string{"x"}, 1, quantityPredicate(0))
	registerValueFunction("QuantityCalculations::isUnit", []string{"x"}, 1, quantityPredicate(1))
	registerValueFunction("QuantityCalculations::abs", []string{"x"}, 1, quantityMagnitudeUnary(numericAbs))
	registerValueFunction("QuantityCalculations::+", []string{"x", "y"}, 1, quantityAdditive(ast.OpAdd))
	registerValueFunction("QuantityCalculations::-", []string{"x", "y"}, 1, quantityAdditive(ast.OpSub))
	registerValueFunction("QuantityCalculations::*", []string{"x", "y"}, 2, quantityMultiplicative(ast.OpMul))
	registerValueFunction("QuantityCalculations::/", []string{"x", "y"}, 2, quantityMultiplicative(ast.OpDiv))
	registerValueFunction("QuantityCalculations::**", []string{"x", "y"}, 2, quantityPower)
	registerValueFunction("QuantityCalculations::^", []string{"x", "y"}, 2, quantityPower)
	registerValueFunction("QuantityCalculations::<", []string{"x", "y"}, 2, quantityComparison(ast.OpLt))
	registerValueFunction("QuantityCalculations::>", []string{"x", "y"}, 2, quantityComparison(ast.OpGt))
	registerValueFunction("QuantityCalculations::<=", []string{"x", "y"}, 2, quantityComparison(ast.OpLe))
	registerValueFunction("QuantityCalculations::>=", []string{"x", "y"}, 2, quantityComparison(ast.OpGe))
	registerValueFunction("QuantityCalculations::==", []string{"x", "y"}, 2, quantityEquality)
	registerValueFunction("QuantityCalculations::max", []string{"x", "y"}, 2, quantityExtremum(ast.OpGt))
	registerValueFunction("QuantityCalculations::min", []string{"x", "y"}, 2, quantityExtremum(ast.OpLt))
	registerValueFunction("QuantityCalculations::sqrt", []string{"x"}, 1, quantitySqrt)
	registerValueFunction("QuantityCalculations::floor", []string{"x"}, 1, quantityMagnitudeUnary(floorToInteger))
	registerValueFunction("QuantityCalculations::round", []string{"x"}, 1, quantityMagnitudeUnary(roundToInteger))
	registerValueFunction("QuantityCalculations::ToString", []string{"x"}, 1, quantityToString)
	registerValueFunction("QuantityCalculations::ToInteger", []string{"x"}, 1, quantityToInteger)
	registerValueFunction("QuantityCalculations::ToRational", []string{"x"}, 1, quantityToReal)
	registerValueFunction("QuantityCalculations::ToReal", []string{"x"}, 1, quantityToReal)
	registerValueFunction("QuantityCalculations::ToDimensionOneValue", []string{"x"}, 1, toDimensionOneValue)
	registerValueFunction("QuantityCalculations::sum", []string{"collection"}, 0, quantityAggregate(ast.OpAdd))
	registerValueFunction("QuantityCalculations::product", []string{"collection"}, 0, quantityAggregate(ast.OpMul))
	registerValueFunction("QuantityCalculations::ConvertQuantity", []string{"x", "targetMRef"}, 2, convertQuantity)
}

// registerMeasurementRefCalculations registers MeasurementRefCalculations over
// the measurement reference and coordinate frame values.
func registerMeasurementRefCalculations() {
	registerValueFunction("MeasurementRefCalculations::*", []string{"x", "y"}, 2, measurementRefArithmetic(ast.OpMul))
	registerValueFunction("MeasurementRefCalculations::/", []string{"x", "y"}, 2, measurementRefArithmetic(ast.OpDiv))
	registerValueFunction("MeasurementRefCalculations::**", []string{"x", "y"}, 2, measurementRefArithmetic(ast.OpPow))
	registerValueFunction("MeasurementRefCalculations::^", []string{"x", "y"}, 2, measurementRefArithmetic(ast.OpPow))
	registerValueFunction("MeasurementRefCalculations::CoordinateFrame*", []string{"x", "y"}, 2, frameArithmetic(ast.OpMul))
	registerValueFunction("MeasurementRefCalculations::CoordinateFrame/", []string{"x", "y"}, 2, frameArithmetic(ast.OpDiv))
	registerValueFunction("MeasurementRefCalculations::ToString", []string{"x"}, 1, measurementRefToString)
}

// anonymous is an `in : Type` parameter: it has no name and binds by position only.
const anonymous = ""

// registerVectorCalculations registers VectorCalculations over vectors and vector
// quantities; those needing a tensor are named.
// A parameter declared `in : Type` under a VectorFunctions general takes that
// general's parameter name (KerML implicit redefinition); otherwise it is anonymous.
func registerVectorCalculations() {
	registerValueFunction("VectorCalculations::[", []string{"elements", "mRef"}, 2, vectorQuantityOf)
	registerValueFunction("VectorCalculations::isZeroVectorQuantity", []string{"v"}, 1, vectorIsZero)
	registerValueFunction("VectorCalculations::isUnitVectorQuantity", []string{anonymous}, 1, vectorIsUnit)
	registerValueFunction("VectorCalculations::+", []string{"v", "w"}, 2, vectorAdd)
	registerValueFunction("VectorCalculations::-", []string{"v", "w"}, 2, vectorSubtract)
	registerValueFunction("VectorCalculations::scalarVectorMult", []string{"x", "v"}, 2, scalarVectorMult)
	registerValueFunction("VectorCalculations::vectorScalarMult", []string{"v", "x"}, 2, vectorScalarMult)
	registerValueFunction("VectorCalculations::scalarQuantityVectorMult", []string{anonymous, anonymous}, 2, scalarQuantityVectorMult)
	registerValueFunction("VectorCalculations::vectorScalarQuantityMult", []string{anonymous, anonymous}, 2, vectorScalarQuantityMult)
	registerValueFunction("VectorCalculations::vectorScalarDiv", []string{"v", "x"}, 2, vectorScalarDiv)
	registerValueFunction("VectorCalculations::vectorScalarQuantityDiv", []string{anonymous, anonymous}, 2, vectorScalarQuantityDiv)
	registerValueFunction("VectorCalculations::inner", []string{"v", "w"}, 2, vectorInner)
	registerUnevaluable("VectorCalculations::outer", []declaredParam{param(anonymous), param(anonymous)}, noOuterProductType)
	registerValueFunction("VectorCalculations::norm", []string{"v"}, 1, vectorNorm)
	registerValueFunction("VectorCalculations::angle", []string{"v", "w"}, 2, vectorAngle)
	registerValueFunction("VectorCalculations::transform", []string{"transformation", "sourceVector"}, 2, transformVector)
}

// registerTensorCalculations registers TensorCalculations over the tensor quantity
// representation; the products the library states no contraction for, and the
// transformation needing a frame, are named.
func registerTensorCalculations() {
	registerValueFunction("TensorCalculations::[", []string{"elements", "mRef"}, 2, tensorOf)
	registerValueFunction("TensorCalculations::isZeroTensorQuantity", []string{"x"}, 1, tensorIsZero)
	registerValueFunction("TensorCalculations::isUnitTensorQuantity", []string{"x"}, 1, tensorIsUnit)
	registerValueFunction("TensorCalculations::+", []string{"x", "y"}, 2, tensorAdditive(ast.OpAdd))
	registerValueFunction("TensorCalculations::-", []string{"x", "y"}, 2, tensorAdditive(ast.OpSub))
	registerValueFunction("TensorCalculations::scalarTensorMult", []string{anonymous, anonymous}, 2, scalarTensorMult(0, 1))
	registerValueFunction("TensorCalculations::TensorScalarMult", []string{anonymous, anonymous}, 2, scalarTensorMult(1, 0))
	registerValueFunction("TensorCalculations::scalarQuantityTensorMult", []string{anonymous, anonymous}, 2, scalarQuantityTensorMult(0, 1))
	registerValueFunction("TensorCalculations::TensorScalarQuantityMult", []string{anonymous, anonymous}, 2, scalarQuantityTensorMult(1, 0))
	registerUnevaluable("TensorCalculations::tensorVectorMult", []declaredParam{param(anonymous), param(anonymous)}, noContractionConvention(tensorVectorMultDecl))
	registerUnevaluable("TensorCalculations::vectorTensorMult", []declaredParam{param(anonymous), param(anonymous)}, noContractionConvention(vectorTensorMultDecl))
	registerUnevaluable("TensorCalculations::tensorTensorMult", []declaredParam{param(anonymous), param(anonymous)}, noContractionConvention(tensorTensorMultDecl))
	registerUnevaluable("TensorCalculations::transform",
		[]declaredParam{param("transformation"), param("sourceTensor")}, noTensorTransformation)
}

// registerAngleFunction adds a trigonometric function of an angle in radians,
// which also accepts the angle as a quantity: `sin(30 ['°'])` is sin(π/6).
// Over numbers it is scalar, so the compiled calc tier may call it unboxed.
func registerAngleFunction(name string, params []string, apply func([]semantics.Value) (semantics.Value, error)) {
	registerValueFunction(name, params, len(params), angleScalars(params, apply))
	libraryFunctions[name].scalar = true
}

// angleScalars adapts a numeric implementation so that each argument may be a
// number of radians or an angle quantity, read as the radians it reduces to; a
// quantity in any other unit, of dimension one (bit, sr) or not, is not an angle.
func angleScalars(params []string, apply func([]semantics.Value) (semantics.Value, error)) libraryApply {
	return func(name string, ctx *Context, args []Value) (Value, error) {
		values := make([]semantics.Value, len(args))
		for i, arg := range args {
			num, ok := angleArgument(ctx, arg)
			if !ok {
				return Value{}, fmt.Errorf(
					"%w: function %s parameter %s requires a number of radians or an angle quantity, got %s",
					ErrTypeMismatch, name, parameterLabel(params, i), describeValue(arg),
				)
			}
			values[i] = num
		}
		result, err := apply(values)
		if err != nil {
			return Value{}, fmt.Errorf("function %s: %w", name, err)
		}
		return Value{Kind: ValConst, Const: result}, nil
	}
}

// angleArgument reads an angle in radians: a number, a ratio that cancelled to
// a number (`m/m`), or a quantity in an angular unit converted to radians.
func angleArgument(ctx *Context, val Value) (semantics.Value, bool) {
	switch val.Kind {
	case ValConst:
		return val.Const, val.Const.IsNumeric()
	case ValQuantity:
		q := val.Quantity()
		if !q.Unit.Term.Dimensionless() || !(q.Unit.Product.IsEmpty() || isAngleUnit(ctx, q.Unit.Product)) {
			return semantics.Value{}, false
		}
		if q.Unit.Term.Scale == semantics.UnitScale(1) {
			return q.Num, true
		}
		return semantics.Value{Kind: semantics.ValReal, Real: q.BaseMagnitude()}, true
	}
	return semantics.Value{}, false
}

// isAngleUnit reports whether the product is one angular-measure unit (rad, °),
// as the model classifies it; `rad**2` or an unresolved unit is not an angle.
func isAngleUnit(ctx *Context, product semantics.UnitProduct) bool {
	if ctx == nil || ctx.model == nil || len(product.Powers) != 1 {
		return false
	}
	f := product.Powers[0]
	return f.Exponent == 1 && ctx.model.IsAngularMeasureUnit(f.Unit)
}

// quantityArg reads a ScalarQuantityValue argument: a quantity, or a number,
// which is how the runtime holds a quantity of dimension one (ToDimensionOneValue).
func quantityArg(name, param string, val Value) (*Quantity, error) {
	q, ok := asQuantity(val)
	if !ok {
		return nil, fmt.Errorf("%w: function %s parameter %q requires a quantity, got %s",
			ErrTypeMismatch, name, param, describeValue(val))
	}
	return q, nil
}

// quantityPredicate is isZero or isUnit: whether the magnitude is the given
// number, as the library bodies test `x.num`.
func quantityPredicate(magnitude float64) libraryApply {
	return func(name string, _ *Context, args []Value) (Value, error) {
		q, err := quantityArg(name, "x", args[0])
		if err != nil {
			return Value{}, err
		}
		return boolValue(toReal(q.Num) == magnitude), nil
	}
}

// quantityMagnitudeUnary applies a numeric function to the magnitude, keeping
// the unit: abs, floor and round.
func quantityMagnitudeUnary(apply func([]semantics.Value) (semantics.Value, error)) libraryApply {
	return func(name string, _ *Context, args []Value) (Value, error) {
		q, err := quantityArg(name, "x", args[0])
		if err != nil {
			return Value{}, err
		}
		num, err := apply([]semantics.Value{q.Num})
		if err != nil {
			return Value{}, fmt.Errorf("function %s: %w", name, err)
		}
		return inUnit(num, q.Unit)
	}
}

// quantityAdditive is '+' or '-': the binary operator, or with y omitted the
// unary one.
func quantityAdditive(op ast.OperatorKind) libraryApply {
	return func(name string, ctx *Context, args []Value) (Value, error) {
		x, err := quantityArg(name, "x", args[0])
		if err != nil {
			return Value{}, err
		}
		if argumentOmitted(args[1]) {
			if op == ast.OpSub {
				val, err := ctx.negateQuantity(x)
				return val, functionError(name, err)
			}
			return inUnit(x.Num, x.Unit)
		}
		y, err := quantityArg(name, "y", args[1])
		if err != nil {
			return Value{}, err
		}
		val, err := ctx.addQuantities(op, x, y)
		return val, functionError(name, err)
	}
}

// quantityMultiplicative is '*' or '/', composing the operands' units.
func quantityMultiplicative(op ast.OperatorKind) libraryApply {
	return func(name string, ctx *Context, args []Value) (Value, error) {
		x, y, err := quantityArgs(name, args)
		if err != nil {
			return Value{}, err
		}
		val, err := ctx.scaleQuantities(op, x, y)
		return val, functionError(name, err)
	}
}

// quantityPower is '**' and '^': the quantity raised to a Real exponent.
func quantityPower(name string, ctx *Context, args []Value) (Value, error) {
	x, err := quantityArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	y, err := scalarArg(name, "y", args[1])
	if err != nil {
		return Value{}, err
	}
	val, err := ctx.powQuantity(x, y)
	return val, functionError(name, err)
}

// quantityComparison is one of the four orderings, in the left operand's unit.
func quantityComparison(op ast.OperatorKind) libraryApply {
	return func(name string, ctx *Context, args []Value) (Value, error) {
		x, y, err := quantityArgs(name, args)
		if err != nil {
			return Value{}, err
		}
		val, err := ctx.compareQuantities(op, x, y)
		return val, functionError(name, err)
	}
}

// quantityEquality is '==', in the left operand's unit.
func quantityEquality(name string, ctx *Context, args []Value) (Value, error) {
	x, y, err := quantityArgs(name, args)
	if err != nil {
		return Value{}, err
	}
	val, err := ctx.equalQuantities(ast.OpEq, x, y)
	return val, functionError(name, err)
}

// quantityExtremum is max (op '>') or min (op '<'): the winning operand as written,
// `max(1 [m], 200 [cm])` being `200 [cm]`, and the first where the two are equal.
// Its result names a unit, so unlike a comparison a bare zero adopts none here.
func quantityExtremum(op ast.OperatorKind) libraryApply {
	return func(name string, ctx *Context, args []Value) (Value, error) {
		x, y, err := quantityArgs(name, args)
		if err != nil {
			return Value{}, err
		}
		if isBareZero(*x) || isBareZero(*y) {
			if _, err := y.ConvertTo(x.Unit); err != nil {
				return Value{}, fmt.Errorf("function %s: %w", name, err)
			}
		}
		yWins, err := ctx.compareQuantities(op, y, x)
		if err != nil {
			return Value{}, fmt.Errorf("function %s: %w", name, err)
		}
		if yWins.Const.Bool {
			return args[1], nil
		}
		return args[0], nil
	}
}

// quantitySqrt is sqrt: the root of the magnitude in the root of the unit.
func quantitySqrt(name string, ctx *Context, args []Value) (Value, error) {
	x, err := quantityArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	val, err := ctx.sqrtQuantity(x)
	return val, functionError(name, err)
}

// quantityToString renders the quantity as the REPL prints it: `1.5 [m/s]`.
func quantityToString(name string, _ *Context, args []Value) (Value, error) {
	x, err := quantityArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	val, err := inUnit(x.Num, x.Unit)
	if err != nil {
		return Value{}, err
	}
	return NewStringValue(FormatValue(val)), nil
}

// quantityToInteger is ToInteger: the magnitude as an Integer, truncated toward
// zero as RealFunctions::ToInteger is; the library declares floor and round beside it.
func quantityToInteger(name string, _ *Context, args []Value) (Value, error) {
	x, err := quantityArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	if x.Num.Kind == semantics.ValInt {
		return Value{Kind: ValConst, Const: x.Num}, nil
	}
	num, err := integerResult(math.Trunc(x.Num.Real))
	if err != nil {
		return Value{}, fmt.Errorf("function %s: %w", name, err)
	}
	return Value{Kind: ValConst, Const: num}, nil
}

// quantityToReal is ToReal and ToRational: the magnitude as a Real.
func quantityToReal(name string, _ *Context, args []Value) (Value, error) {
	x, err := quantityArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: toReal(x.Num)}}, nil
}

// toDimensionOneValue is ToDimensionOneValue: a Real as a quantity of dimension
// one, which a bare number already is to every quantity operation.
func toDimensionOneValue(name string, _ *Context, args []Value) (Value, error) {
	x, err := scalarArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	return Value{Kind: ValConst, Const: x}, nil
}

// quantityAggregate is sum or product over quantities, folded in the first element's
// unit; an empty collection has no unit, so it is the dimensionless 0 or 1.
func quantityAggregate(op ast.OperatorKind) libraryApply {
	return func(name string, ctx *Context, args []Value) (Value, error) {
		return ctx.aggregate(name, args, op, false)
	}
}

// vectorIsUnit is VectorCalculations::isUnitVectorQuantity: whether the norm of
// its one anonymous parameter is one.
func vectorIsUnit(name string, _ *Context, args []Value) (Value, error) {
	v, err := readVector(name, positionLabel(0), args[0])
	if err != nil {
		return Value{}, err
	}
	reals, err := v.reals(name)
	if err != nil {
		return Value{}, err
	}
	return boolValue(euclideanNorm(reals) == 1), nil
}

// quantityArgs reads the two ScalarQuantityValue parameters x and y.
func quantityArgs(name string, args []Value) (*Quantity, *Quantity, error) {
	x, err := quantityArg(name, "x", args[0])
	if err != nil {
		return nil, nil, err
	}
	y, err := quantityArg(name, "y", args[1])
	if err != nil {
		return nil, nil, err
	}
	return x, y, nil
}

// functionError names the function in the error of a quantity operation.
func functionError(name string, err error) error {
	if err != nil {
		return fmt.Errorf("function %s: %w", name, err)
	}
	return nil
}
