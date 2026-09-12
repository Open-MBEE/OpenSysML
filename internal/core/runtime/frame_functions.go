package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// frameIndex evaluates the index of `num [ref]` when it names or composes a
// coordinate frame or measurement scale rather than a unit; false for any other index.
func (ec *EvalContext) frameIndex(index ast.Node) (*CoordinateFrame, bool, error) {
	switch n := index.(type) {
	case *ast.FeatureReference, *ast.QualifiedName, *ast.FeatureChainExpr:
		if ec.ctx.model.resolver == nil {
			return nil, false, nil
		}
		sym, ok := ec.ctx.resolveTarget(ec.scope, index)
		if !ok || sym == nil {
			return nil, false, nil
		}
		if alias, ok := ec.ctx.resolveAliasTarget(sym); ok {
			sym = alias
		}
		if !ec.ctx.isFrameType(ec.ctx.extractType(sym)) {
			return nil, false, nil
		}
	case *ast.OperatorExpr:
		// `spatialCF / s`: a frame the checker proves 'CoordinateFrame/' composes.
		if ec.ctx.model.semantics.CoordinateFrameExprType(ec.scope, n) == nil {
			return nil, false, nil
		}
	default:
		return nil, false, nil
	}
	val, err := ec.Eval(index)
	if err != nil {
		return nil, true, err
	}
	if val.Kind != ValCoordinateFrame {
		return nil, true, fmt.Errorf("%w: %s evaluates to %s, want a coordinate frame or a measurement scale",
			ErrNotAQuantity, semantics.UnitExprText(index), describeValue(val))
	}
	return val.CoordinateFrame(), true, nil
}

// framedQuantity is `elements [frame]` (VectorCalculations::'['): a vector quantity
// over the frame, one component per axis; over a scale, `num [scale]` is the scalar
// quantity whose magnitude is a point on the scale.
func (ec *EvalContext) framedQuantity(n *ast.IndexExpr, frame *CoordinateFrame) (Value, error) {
	magnitude, err := ec.valueOperand(n.Operand)
	if err != nil {
		return Value{}, err
	}
	if frame.IsScale() && magnitude.Kind == ValConst && magnitude.Const.IsNumeric() {
		return NewQuantityValue(&Quantity{Num: magnitude.Const, Unit: frame.Axes[0]}), nil
	}
	elements, ok := numericElements(magnitude)
	if !ok {
		size, sized := frame.FlattenedSize()
		if !sized {
			return Value{}, frame.flattenedSizeError(frame.Name())
		}
		return Value{}, fmt.Errorf("%w: magnitude of a quantity over the coordinate frame %s is %s, want %d numbers, one per axis",
			ErrNotAQuantity, frame.Name(), describeValue(magnitude), size)
	}
	return ec.ctx.framedVectorQuantity(elements, frame)
}

// numericElements is the numbers a vector or a sequence of numbers holds.
func numericElements(val Value) ([]semantics.Value, bool) {
	switch val.Kind {
	case ValVector:
		return val.Vector().Elements, true
	case ValConst:
		if val.Const.IsNumeric() {
			return []semantics.Value{val.Const}, true
		}
	case ValSequence:
		elements := val.Sequence().Elements()
		nums := make([]semantics.Value, len(elements))
		for i, el := range elements {
			if el.Kind != ValConst || !el.Const.IsNumeric() {
				return nil, false
			}
			nums[i] = el.Const
		}
		return nums, true
	}
	return nil, false
}

// framedVectorQuantity builds the vector quantity of num over frame, which
// VectorCalculations::'[' sizes as `n = mRef.flattenedSize`.
func (ctx *Context) framedVectorQuantity(num []semantics.Value, frame *CoordinateFrame) (Value, error) {
	size, ok := frame.FlattenedSize()
	if !ok {
		return Value{}, frame.flattenedSizeError("VectorCalculations::'[': " + frame.Name())
	}
	if int64(len(num)) != size {
		return Value{}, fmt.Errorf(
			"%w: VectorCalculations::'[': %d elements over the coordinate frame %s, whose flattenedSize is %d; elements: Number[1..n] with n = mRef.flattenedSize",
			ErrMultiplicityViolation, len(num), frame.Name(), size)
	}
	checked, err := ctx.checkedComponents(num)
	if err != nil {
		return Value{}, err
	}
	return NewFramedVectorQuantityValue(checked, frame), nil
}

// composeFrame is `frame * unit` or `frame / unit` (MeasurementRefCalculations::
// 'CoordinateFrame*' and 'CoordinateFrame/'): the frame whose every axis is the
// axis times or over the unit, of the same dimensions; false for other operands.
func (ctx *Context) composeFrame(op ast.OperatorKind, left, right Value) (Value, bool, error) {
	if left.Kind != ValCoordinateFrame || (op != ast.OpMul && op != ast.OpDiv) {
		return Value{}, false, nil
	}
	frame := left.CoordinateFrame()
	if right.Kind != ValMeasurementRef {
		return Value{}, true, fmt.Errorf("%w: operator '%s' is not defined for a coordinate frame and %s; MeasurementRefCalculations::'CoordinateFrame%s' takes a MeasurementUnit",
			ErrTypeMismatch, op.String(), describeValue(right), op.String())
	}
	if frame.IsScale() {
		return Value{}, true, fmt.Errorf("%w: MeasurementRefCalculations::'CoordinateFrame%s': %s is a measurement scale, whose points are on the scale and not in a unit to compose",
			ErrUnevaluableLibraryFunction, op.String(), frame.Name())
	}
	unit := right.MeasurementRef().Unit
	axes := make([]Unit, len(frame.Axes))
	for i, axis := range frame.Axes {
		if scale, ok := ctx.model.semantics.MeasurementScaleOf(axis.Term); ok {
			return Value{}, true, fmt.Errorf("%w: MeasurementRefCalculations::'CoordinateFrame%s': axis %d of %s is on the measurement scale %s, whose points are not in a unit to compose",
				ErrUnevaluableLibraryFunction, op.String(), i+1, frame.Name(), unitSymbolName(scale))
		}
		if op == ast.OpMul {
			axes[i] = Unit{Product: axis.Product.Times(unit.Product), Term: axis.Term.Times(unit.Term)}
		} else {
			axes[i] = Unit{Product: axis.Product.DividedBy(unit.Product), Term: axis.Term.DividedBy(unit.Term)}
		}
		axes[i] = measurementRefOf(axes[i]).MeasurementRef().Unit
	}
	// The composed frame has the axes' units and dimensions only: the declaration's
	// placement locates that frame, not one over other units.
	composed := &CoordinateFrame{
		Dimensions: append([]int64(nil), frame.Dimensions...),
		Axes:       axes,
		Text:       frame.Name() + " " + op.String() + " " + right.MeasurementRef().String(),
	}
	return NewCoordinateFrameValue(composed), true, nil
}

// frameArithmetic is MeasurementRefCalculations::'CoordinateFrame*' and
// 'CoordinateFrame/' called by name: x a CoordinateFrame, y a MeasurementUnit.
func frameArithmetic(op ast.OperatorKind) libraryApply {
	return func(name string, ctx *Context, args []Value) (Value, error) {
		x, y := soleElement(args[0]), soleElement(args[1])
		if x.Kind != ValCoordinateFrame {
			return Value{}, fmt.Errorf("%w: function %s parameter %q requires a coordinate frame, a usage typed CoordinateFrame with its mRefs, got %s",
				ErrTypeMismatch, name, "x", describeValue(x))
		}
		if _, err := measurementRefArg(name, "y", y); err != nil {
			return Value{}, err
		}
		val, _, err := ctx.composeFrame(op, x, y)
		return val, functionError(name, err)
	}
}

// vectorQuantityOf is VectorCalculations::'[' called by name: the numbers
// `elements` over the coordinate frame `mRef`, one per axis.
func vectorQuantityOf(name string, ctx *Context, args []Value) (Value, error) {
	if args[1].Kind != ValCoordinateFrame {
		return Value{}, fmt.Errorf("%w: function %s parameter %q requires a coordinate frame, a usage typed CoordinateFrame with its mRefs, got %s",
			ErrTypeMismatch, name, "mRef", describeValue(args[1]))
	}
	elements, ok := numericElements(args[0])
	if !ok {
		return Value{}, fmt.Errorf("%w: function %s parameter %q requires numbers, one per axis, got %s",
			ErrTypeMismatch, name, "elements", describeValue(args[0]))
	}
	return ctx.framedVectorQuantity(elements, args[1].CoordinateFrame())
}

// classifiedFrame is a composed frame written to a feature typed by a frame type,
// taking the declaration's identity, type and name (`velocityCF = spatialCF / s`);
// any other value is itself.
func (ctx *Context) classifiedFrame(decl *symbols.Symbol, val Value) Value {
	if val.Kind != ValCoordinateFrame || decl == nil {
		return val
	}
	frame := val.CoordinateFrame()
	if frame.Object != 0 || frame.Decl != nil {
		return val
	}
	typ := ctx.extractType(decl)
	if !ctx.isFrameType(typ) {
		return val
	}
	named := *frame
	named.Decl, named.Type, named.Text = decl, typ, symbolText(decl)
	return NewCoordinateFrameValue(&named)
}

// frameValueType is a frame's declared type, or CoordinateFrame for one composed
// by `CoordinateFrame*` or `CoordinateFrame/` and bound to no declaration.
func (ctx *Context) frameValueType(frame *CoordinateFrame) (*symbols.Symbol, error) {
	if frame != nil && frame.Type != nil {
		return frame.Type, nil
	}
	return ctx.loadedLibraryType(coordinateFrameTypeFQN)
}

// transformationValueType is a transformation's declared type.
func (ctx *Context) transformationValueType(t *CoordinateTransformation) (*symbols.Symbol, error) {
	if t != nil && t.Type != nil {
		return t.Type, nil
	}
	return ctx.loadedLibraryType(transformationTypeFQN)
}

// frameConforms judges a frame or scale written to a feature: by its type where
// it has one, else as the checker judges `cf / u` — a CoordinateFrame whose
// dimensions and axes the feature's type may constrain.
func (ctx *Context) frameConforms(frame *CoordinateFrame, declared *symbols.Symbol) (bool, string, error) {
	found, refused := ctx.frameRefusal(frame, declared)
	if !refused {
		return true, "", nil
	}
	return false, fmt.Sprintf("cannot write the coordinate frame %s, %s, to a feature typed by %s",
		frame, found, symbolText(declared)), nil
}

// frameRefusal says what a frame is that no feature of the declared type may hold:
// its type, or what the checker knows of a composed frame; false where it conforms.
func (ctx *Context) frameRefusal(frame *CoordinateFrame, declared *symbols.Symbol) (string, bool) {
	if frame.Type != nil {
		if ctx.modelConforms(frame.Type, declared) {
			return "", false
		}
		return "a " + symbolText(frame.Type), true
	}
	c := ctx.model.semantics.ComposedFrameConforms(ctx.composedFrame(frame), declared)
	if !c.Known || c.Holds {
		return "", false
	}
	return c.Found, true
}

// framedQuantityConforms judges a vector quantity written over a frame against the
// frames the declared type's `mRef` admits, as the checker judges `(1, 2, 3) [cf]`.
func (ctx *Context) framedQuantityConforms(value Value, declared *symbols.Symbol) (bool, string) {
	frame := value.VectorQuantity().Frame
	for _, admitted := range ctx.model.semantics.AdmittedMeasurementRefs(declared) {
		if found, refused := ctx.frameRefusal(frame, admitted); refused {
			return false, fmt.Sprintf("cannot write %s (a vector quantity over the coordinate frame %s, %s) to a feature typed by %s, whose mRef admits %s",
				FormatValue(value), frame, found, symbolText(declared), symbolText(admitted))
		}
	}
	return true, ""
}

// composedFrame is what the checker knows of a frame `cf * u` composes: its
// dimensions and the dimension each axis measures in, where the units have one.
func (ctx *Context) composedFrame(frame *CoordinateFrame) semantics.ComposedFrame {
	composed := semantics.ComposedFrame{Dimensions: frame.Dimensions, HasDimensions: true}
	axes := make([]semantics.UnitTerm, 0, len(frame.Axes))
	for _, axis := range frame.Axes {
		dim, ok := ctx.model.semantics.DimensionOfUnit(axis.Term)
		if !ok {
			return composed
		}
		axes = append(axes, dim.Term)
	}
	composed.AxisDimensions = axes
	return composed
}

// transformationConforms judges a transformation written to a feature by its type.
func (ctx *Context) transformationConforms(t *CoordinateTransformation, declared *symbols.Symbol) (bool, string, error) {
	typ := t.Type
	if typ == nil {
		var err error
		if typ, err = ctx.loadedLibraryType(transformationTypeFQN); err != nil {
			return false, "", err
		}
	}
	if ctx.modelConforms(typ, declared) {
		return true, "", nil
	}
	return false, fmt.Sprintf("cannot write the coordinate transformation %s, a %s, to a feature typed by %s",
		t, symbolText(typ), symbolText(declared)), nil
}

// Features of a coordinate frame, scale or transformation answered from the value.
const (
	frameTransformationFeature  = "transformation"
	scaleUnitFeature            = "unit"
	transformationSourceFeature = "source"
	transformationTargetFeature = "target"
)

// frameFeature answers a frame's Array and TensorMeasurementReference features
// from the value; a member the value does not hold is read from its object.
func (ctx *Context) frameFeature(val Value, name string) (Value, bool, error) {
	frame := val.CoordinateFrame()
	switch name {
	case arrayDimensionsFeature:
		return ctx.answerSequence(integerValues(frame.Dimensions))
	case arrayRankFeature:
		return integerValue(int64(len(frame.Dimensions))), true, nil
	case arrayFlattenedSizeFeature:
		size, ok := frame.FlattenedSize()
		if !ok {
			return Value{}, true, frame.flattenedSizeError(frame.Name())
		}
		return integerValue(size), true, nil
	case arrayElementsFeature, mRefMRefsFeature:
		if frame.IsScale() {
			return ctx.answerSequence([]Value{val})
		}
		return ctx.answerSequence(frame.axisRefs())
	case frameTransformationFeature:
		if frame.Transformation == nil {
			return ctx.answerSequence(nil)
		}
		return NewCoordinateTransformationValue(frame.Transformation), true, nil
	case scaleUnitFeature:
		if frame.IsScale() {
			return measurementRefOf(frame.Scale.Unit), true, nil
		}
	}
	return ctx.objectMember(frame.Object, name)
}

// transformationFeature answers a transformation's source and target from the
// value; its other members are read from its object.
func (ctx *Context) transformationFeature(val Value, name string) (Value, bool, error) {
	t := val.CoordinateTransformation()
	switch name {
	case transformationSourceFeature:
		if t.Source == nil {
			return ctx.answerSequence(nil)
		}
		return NewCoordinateFrameValue(t.Source), true, nil
	case transformationTargetFeature:
		if t.Target == nil {
			return ctx.answerSequence(nil)
		}
		return NewCoordinateFrameValue(t.Target), true, nil
	}
	return ctx.objectMember(t.Object, name)
}

// featuringReferenceValue answers a member the enclosing transformation's value
// supplies (`target = that`): the frame it is read through, or none standalone.
func (ec *EvalContext) featuringReferenceValue(sym *symbols.Symbol) (Value, bool, error) {
	for inst := ec.self; inst != nil; inst, _ = inst.Owner() {
		if !ec.ctx.isTransformationType(ec.ctx.objectType(inst)) || !ec.ctx.model.semantics.RestatesFeaturingReference(inst.Type, sym) {
			continue
		}
		val, ok, err := ec.ctx.referenceValueOfObject(inst)
		if !ok || err != nil {
			return Value{}, ok, err
		}
		return ec.ctx.transformationFeature(val, transformationTargetFeature)
	}
	return Value{}, false, nil
}

// objectMember reads a member of the object a value was read from, if it has one
// and the member is stated there.
func (ctx *Context) objectMember(object int64, name string) (Value, bool, error) {
	inst, ok := ctx.instances[object]
	if !ok {
		return Value{}, false, nil
	}
	if _, declared := inst.FeatureValues[name]; !declared {
		return Value{}, false, nil
	}
	fv, err := inst.GetFeatureValue(ctx, name)
	if err != nil {
		return Value{}, true, err
	}
	val, err := ctx.readFeatureValue(fv, name)
	return val, true, err
}
