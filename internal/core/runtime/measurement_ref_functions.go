package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// measurementRefArg reads a ScalarMeasurementReference (MeasurementUnit) parameter.
func measurementRefArg(name, param string, val Value) (*MeasurementRef, error) {
	if val.Kind != ValMeasurementRef || val.MeasurementRef() == nil {
		return nil, fmt.Errorf("%w: function %s parameter %q requires a measurement reference such as SI::m, got %s",
			ErrTypeMismatch, name, param, describeValue(val))
	}
	return val.MeasurementRef(), nil
}

// scalarReferenceArg admits a ScalarMeasurementReference: a unit, or a scale
// read as the reference its points are on.
func scalarReferenceArg(name, param string, val Value) (*MeasurementRef, error) {
	if val.Kind == ValCoordinateFrame {
		frame := val.CoordinateFrame()
		if frame.IsScale() {
			return &MeasurementRef{Unit: frame.Axes[0]}, nil
		}
		return nil, fmt.Errorf("%w: function %s parameter %q requires a ScalarMeasurementReference, got the coordinate frame %s of %d axes",
			ErrTypeMismatch, name, param, frame, len(frame.Axes))
	}
	return measurementRefArg(name, param, val)
}

// quantityOf is QuantityCalculations::'[': the number `num` in the reference `mRef`.
func quantityOf(name string, _ *Context, args []Value) (Value, error) {
	num, err := scalarArg(name, "num", args[0])
	if err != nil {
		return Value{}, err
	}
	ref, err := scalarReferenceArg(name, "mRef", args[1])
	if err != nil {
		return Value{}, err
	}
	return inUnit(num, ref.Unit)
}

// convertQuantity is QuantityCalculations::ConvertQuantity: x expressed in
// targetMRef, a unit commensurable with x's own reference or a scale placed on one.
func convertQuantity(name string, ctx *Context, args []Value) (Value, error) {
	x, err := quantityArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	target, err := ctx.conversionTarget(name, args[1])
	if err != nil {
		return Value{}, err
	}
	val, err := quantityResult(ctx.convertToReference(*x, target))
	return val, functionError(name, err)
}

// measurementRefArithmetic is MeasurementRefCalculations::'*', '/', '**' and '^'
// over MeasurementUnits: the composed unit, as a quantity's unit composes.
func measurementRefArithmetic(op ast.OperatorKind) libraryApply {
	return func(name string, ctx *Context, args []Value) (Value, error) {
		if _, err := measurementRefArg(name, "x", args[0]); err != nil {
			return Value{}, err
		}
		if op == ast.OpPow {
			if _, err := scalarArg(name, "y", args[1]); err != nil {
				return Value{}, err
			}
		} else if _, err := measurementRefArg(name, "y", args[1]); err != nil {
			return Value{}, err
		}
		ref, ok, err := ctx.composeMeasurementRefs(op, args[0], args[1])
		if !ok {
			return Value{}, fmt.Errorf("%w: function %s is not defined over %s and %s",
				ErrTypeMismatch, name, describeValue(args[0]), describeValue(args[1]))
		}
		return ref, err
	}
}

// measurementRefToString is MeasurementRefCalculations::ToString: the symbol
// the reference is written by, `m` or `km/h`.
func measurementRefToString(name string, _ *Context, args []Value) (Value, error) {
	ref, err := scalarReferenceArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	return NewStringValue(ref.String()), nil
}
