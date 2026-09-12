package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// TensorCalculations: the calculations the library defines over a TensorQuantityValue's
// data. Those it gives no convention for stay unevaluable.

// Reasons the TensorCalculations the library leaves undetermined are reported with,
// each quoting the declaration that determines nothing more.
const (
	tensorVectorMultDecl = "calc def tensorVectorMult { in : TensorQuantityValue[1]; in : VectorQuantityValue[1]; return : VectorQuantityValue[1]; }"
	vectorTensorMultDecl = "calc def vectorTensorMult { in : VectorQuantityValue[1]; in : TensorQuantityValue[1]; return : VectorQuantityValue[1]; }"
	tensorTensorMultDecl = "calc def tensorTensorMult { in : TensorQuantityValue[1]; in : TensorQuantityValue[1]; return : TensorQuantityValue[1]; }"
	noOuterProductType   = "the declaration `calc def outer { in : VectorQuantityValue[1]; in : VectorQuantityValue[1]; return : VectorQuantityValue[1]; }` " +
		"returns a VectorQuantityValue, and the outer product of two vectors is a tensor of order two, which no VectorQuantityValue is"
	noTensorTransformation = "the declaration `calc def transform { in transformation : CoordinateTransformation; in sourceTensor : TensorQuantityValue; " +
		"return targetTensor : TensorQuantityValue; }` has no body, and neither TensorCalculations nor the Kernel says how a " +
		"CoordinateTransformation acts on a tensor's contravariant and covariant indices: the runtime transforms a vector quantity " +
		"(VectorCalculations::transform), and a tensor quantity carries a unit per component and no source frame"
)

// noContractionConvention is the reason a tensor product with the given declaration is not evaluable.
func noContractionConvention(decl string) string {
	return "the declaration `" + decl + "` states only the parameter and result types; neither TensorCalculations nor " +
		"the Kernel says which indices contract or how the operands' contravariantOrder and covariantOrder combine, " +
		"so no product is determined"
}

// tensorOperand is a TensorQuantityValue argument of any order as the calculations
// read it: a scalar quantity is one of order 0, a vector quantity of order 1.
type tensorOperand struct {
	dims  []int64
	num   []semantics.Value
	units []Unit
	mRef  int64
}

func (t tensorOperand) size() int { return len(t.num) }

// component is the scalar quantity at one row-major offset.
func (t tensorOperand) component(i int) *Quantity {
	return &Quantity{Num: t.num[i], Unit: t.units[i]}
}

// readTensor reads a TensorQuantityValue parameter by the library's subtype chain:
// a tensor, vector or scalar quantity. A bare number or vector is no quantity.
func readTensor(name, label string, val Value) (tensorOperand, error) {
	switch val.Kind {
	case ValTensorQuantity:
		tq := val.TensorQuantity()
		return tensorOperand{dims: tq.Dimensions, num: tq.Num, units: tq.Units, mRef: tq.MRef}, nil
	case ValVectorQuantity:
		vq := val.VectorQuantity()
		return tensorOperand{dims: []int64{int64(vq.Dimension())}, num: vq.Num, units: vq.Units}, nil
	case ValQuantity:
		q := val.Quantity()
		return tensorOperand{dims: []int64{}, num: []semantics.Value{q.Num}, units: []Unit{q.Unit}}, nil
	}
	return tensorOperand{}, fmt.Errorf(
		"%w: function %s parameter %s requires a TensorQuantityValue (a tensor, vector or scalar quantity), got %s",
		ErrTypeMismatch, name, label, describeValue(val),
	)
}

// isTensorSubtypeKind is a value of a kind readTensor accepts: a TensorQuantityValue
// of some order, which the library's subtype chain makes every quantity.
func isTensorSubtypeKind(val Value) bool {
	return val.Kind == ValTensorQuantity || val.Kind == ValVectorQuantity || val.Kind == ValQuantity
}

// tensorResult is the components over the operand's shape, of its order: a scalar,
// vector or tensor quantity; all units reducing to one leaves an Array of numbers.
func (ctx *Context) tensorResult(name string, shape tensorOperand, components []Value) (Value, error) {
	switch len(shape.dims) {
	case 0:
		return components[0], nil
	case 1:
		return ctx.vectorResult(name, components)
	}
	num := make([]semantics.Value, len(components))
	units := make([]Unit, len(components))
	withUnit := false
	for i, component := range components {
		q, ok := asQuantity(component)
		if !ok {
			return Value{}, fmt.Errorf("%w: function %s computed %s for a component",
				ErrTypeMismatch, name, describeValue(component))
		}
		num[i], units[i] = q.Num, q.Unit
		withUnit = withUnit || component.Kind == ValQuantity
	}
	checked, err := ctx.checkedComponents(num)
	if err != nil {
		return Value{}, err
	}
	if !withUnit {
		return NewArrayValue(shape.dims, constValues(checked)), nil
	}
	return Value{Kind: ValTensorQuantity, ref: &TensorQuantity{
		Dimensions: shape.dims, Num: checked, Units: units, MRef: shape.mRef,
	}}, nil
}

// tensorOf is TensorCalculations::'[': the numbers `elements`, one per component
// of `mRef`, each in the reference at its row-major position of `mRef.mRefs`.
func tensorOf(name string, ctx *Context, args []Value) (Value, error) {
	elements, err := vectorElements(name, "elements", args[0])
	if err != nil {
		return Value{}, err
	}
	ref, err := ctx.tensorMRefArg(name, "mRef", args[1])
	if err != nil {
		return Value{}, err
	}
	if int64(len(elements)) != ref.FlattenedSize() {
		return Value{}, fmt.Errorf(
			"%w: function %s: %d elements for a reference of dimensions %s: the declaration `in elements: Number[1..n]` "+
				"has `n = mRef.flattenedSize` = %d, and Array binds mRefs to one reference per component",
			ErrMultiplicityViolation, name, len(elements), FormatValue(intSequence(ref.Dimensions)), ref.FlattenedSize(),
		)
	}
	if len(elements) == 0 {
		return Value{}, functionError(name, errEmptyVectorQuantity())
	}
	units := make([]Unit, len(ref.Elements))
	for i, elem := range ref.Elements {
		units[i] = elem.MeasurementRef().Unit
	}
	checked, err := ctx.checkedComponents(elements)
	if err != nil {
		return Value{}, err
	}
	switch ref.Rank() {
	case 0:
		return inUnit(checked[0], units[0])
	case 1:
		return NewVectorQuantityValue(checked, units), nil
	}
	tq := &TensorQuantity{Dimensions: ref.Dimensions, Num: checked, Units: units, MRef: ref.Object}
	if ref.Object != 0 {
		bound, err := ctx.tensorMRefIsBound(name, ref.Object)
		if err != nil {
			return Value{}, err
		}
		tq.IsBound, tq.BoundKnown = bound, true
	} else {
		tq.BoundKnown = true
	}
	return Value{Kind: ValTensorQuantity, ref: tq}, nil
}

// tensorMRefArg reads a TensorMeasurementReference parameter as the Array of its mRefs:
// a shaped reference object, an Array of scalar references, or one scalar reference.
func (ctx *Context) tensorMRefArg(name, param string, val Value) (*Array, error) {
	switch val.Kind {
	case ValMeasurementRef:
		return &Array{Dimensions: []int64{}, Elements: []Value{val}}, nil
	case ValInstance:
		if inst, ok := ctx.instances[val.Instance]; ok {
			shaped, isArray, err := ctx.arrayOfObject(inst)
			if err != nil {
				return nil, functionError(name, err)
			}
			if isArray {
				return ctx.tensorMRefArg(name, param, shaped)
			}
		}
	case ValArray:
		arr := val.Array()
		if arr.Object != 0 {
			inst, ok := ctx.instances[arr.Object]
			if refSym := ctx.librarySymbol(tensorMRefTypeFQN); !ok || refSym == nil || !ctx.modelConforms(ctx.objectType(inst), refSym) {
				return nil, fmt.Errorf("%w: function %s parameter %q requires a %s, got %s",
					ErrTypeMismatch, name, param, tensorMRefTypeFQN, describeValue(val))
			}
		}
		for i, elem := range arr.Elements {
			if elem.Kind != ValMeasurementRef || elem.MeasurementRef() == nil {
				return nil, fmt.Errorf(
					"%w: function %s parameter %q: mRefs element %d is %s, %s declares mRefs: ScalarMeasurementReference[1..*]",
					ErrTypeMismatch, name, param, i+1, describeValue(elem), tensorMRefTypeFQN,
				)
			}
		}
		return arr, nil
	}
	return nil, fmt.Errorf("%w: function %s parameter %q requires a %s with its dimensions and mRefs stated, got %s",
		ErrTypeMismatch, name, param, tensorMRefTypeFQN, describeValue(val))
}

// tensorMRefIsBound is `isBound` of a TensorMeasurementReference object: as the
// object states it, else as its type declares the default (`default false`).
func (ctx *Context) tensorMRefIsBound(name string, object int64) (bool, error) {
	inst := ctx.instances[object]
	elements, stated, err := ctx.objectFeatureElements(inst, mRefIsBoundFeature)
	if err != nil {
		return false, err
	}
	if !stated {
		member, ok := ctx.model.semantics.LookupMember(ctx.objectType(inst), mRefIsBoundFeature)
		if !ok || member == nil {
			return false, fmt.Errorf("%w: function %s: %s declares no isBound", ErrNoSuchFeature, name, symbolText(inst.Type))
		}
		value := ctx.extractDefaultValue(member)
		if value == nil {
			value, _ = ctx.redefinedDefault(member, ctx.objectType(inst))
		}
		if value == nil {
			return false, fmt.Errorf("%w: function %s: %s states no isBound and its type declares no default",
				ErrUnevaluableLibraryFunction, name, symbolText(inst.Type))
		}
		c, ok := ctx.model.semantics.Eval(value)
		if !ok {
			return false, fmt.Errorf("%w: function %s: the default of %s::isBound is not a constant",
				ErrUnevaluableLibraryFunction, name, symbolText(inst.Type))
		}
		elements = []Value{{Kind: ValConst, Const: c}}
	}
	if len(elements) != 1 || elements[0].Kind != ValConst || elements[0].Const.Kind != semantics.ValBool {
		return false, fmt.Errorf("%w: function %s: %s::isBound is %s, %s declares isBound: Boolean[1]",
			ErrTypeMismatch, name, symbolText(inst.Type), FormatValue(sequenceOf(elements)), tensorMRefTypeFQN)
	}
	return elements[0].Const.Bool, nil
}

// tensorIsZero is isZeroTensorQuantity: every component's magnitude is zero.
func tensorIsZero(name string, _ *Context, args []Value) (Value, error) {
	x, err := readTensor(name, `"x"`, args[0])
	if err != nil {
		return Value{}, err
	}
	for _, n := range x.num {
		if asReal(n) != 0 {
			return boolValue(false), nil
		}
	}
	return boolValue(true), nil
}

// tensorIsUnit is isUnitTensorQuantity, answered for the one shape with an identity:
// a square tensor of order two. The bodiless declaration determines no other shape.
func tensorIsUnit(name string, _ *Context, args []Value) (Value, error) {
	x, err := readTensor(name, `"x"`, args[0])
	if err != nil {
		return Value{}, err
	}
	if len(x.dims) != 2 || x.dims[0] != x.dims[1] {
		return Value{}, fmt.Errorf(
			"%w: %s: `calc def isUnitTensorQuantity { in x : TensorQuantityValue[1]; return : Boolean[1]; }` has no body, "+
				"and only a square tensor of order two has an identity; x has dimensions %s",
			ErrUnevaluableLibraryFunction, name, FormatValue(intSequence(x.dims)),
		)
	}
	n := x.dims[0]
	for i := int64(0); i < n; i++ {
		for j := int64(0); j < n; j++ {
			want := 0.0
			if i == j {
				want = 1
			}
			if asReal(x.num[i*n+j]) != want {
				return boolValue(false), nil
			}
		}
	}
	return boolValue(true), nil
}

// tensorShapeMismatch reports two tensors of different dimensions.
func tensorShapeMismatch(name string, x, y tensorOperand) error {
	return fmt.Errorf("%w: function %s: dimensions %s and %s differ; both parameters are TensorQuantityValue[1] of one shape",
		ErrMultiplicityViolation, name, FormatValue(intSequence(x.dims)), FormatValue(intSequence(y.dims)))
}

// combineTensors is '+' or '-' over two tensors of one shape, each component pair
// added in the left component's unit.
func (ctx *Context) combineTensors(name string, op ast.OperatorKind, x, y tensorOperand) (Value, error) {
	if !equalInt64s(x.dims, y.dims) {
		return Value{}, tensorShapeMismatch(name, x, y)
	}
	components := make([]Value, x.size())
	for i := range components {
		component, err := ctx.addQuantities(op, x.component(i), y.component(i))
		if err != nil {
			return Value{}, functionError(name, err)
		}
		components[i] = component
	}
	return ctx.tensorResult(name, x, components)
}

// tensorAdditive is TensorCalculations::'+' and '-', two TensorQuantityValue[1]
// parameters named x and y by their DataFunctions general.
func tensorAdditive(op ast.OperatorKind) libraryApply {
	return func(name string, ctx *Context, args []Value) (Value, error) {
		x, err := readTensor(name, `"x"`, args[0])
		if err != nil {
			return Value{}, err
		}
		y, err := readTensor(name, `"y"`, args[1])
		if err != nil {
			return Value{}, err
		}
		return ctx.combineTensors(name, op, x, y)
	}
}

// scaleTensor multiplies every component by a scalar quantity, composing each
// component's unit with the scalar's as QuantityCalculations::'*' does.
func (ctx *Context) scaleTensor(name string, x *Quantity, t tensorOperand) (Value, error) {
	components := make([]Value, t.size())
	for i := range components {
		component, err := ctx.scaleQuantities(ast.OpMul, x, t.component(i))
		if err != nil {
			return Value{}, functionError(name, err)
		}
		components[i] = component
	}
	if !x.Unit.None() {
		t.mRef = 0
	}
	return ctx.tensorResult(name, t, components)
}

// scalarTensorMult is TensorCalculations::scalarTensorMult and TensorScalarMult:
// each component multiplied by a Number, its unit kept.
func scalarTensorMult(scalarAt, tensorAt int) libraryApply {
	return func(name string, ctx *Context, args []Value) (Value, error) {
		x, err := scalarArg(name, positionLabel(scalarAt), args[scalarAt])
		if err != nil {
			return Value{}, err
		}
		t, err := readTensor(name, positionLabel(tensorAt), args[tensorAt])
		if err != nil {
			return Value{}, err
		}
		return ctx.scaleTensor(name, &Quantity{Num: x, Unit: semantics.UnitOne()}, t)
	}
}

// scalarQuantityTensorMult is TensorCalculations::scalarQuantityTensorMult and
// TensorScalarQuantityMult: each component multiplied by a scalar quantity.
func scalarQuantityTensorMult(scalarAt, tensorAt int) libraryApply {
	return func(name string, ctx *Context, args []Value) (Value, error) {
		x, err := quantityArg(name, positionLabel(scalarAt), args[scalarAt])
		if err != nil {
			return Value{}, err
		}
		t, err := readTensor(name, positionLabel(tensorAt), args[tensorAt])
		if err != nil {
			return Value{}, err
		}
		return ctx.scaleTensor(name, x, t)
	}
}

// tensorArithmetic is operator notation over a tensor quantity: `+`/`-` are
// TensorCalculations', `*` by a number or scalar quantity its scalar products.
func (ctx *Context) tensorArithmetic(op ast.OperatorKind, left, right Value) (Value, bool, error) {
	lt, rt := left.Kind == ValTensorQuantity, right.Kind == ValTensorQuantity
	if !lt && !rt {
		return Value{}, false, nil
	}
	args := []Value{left, right}
	switch op {
	case ast.OpAdd, ast.OpSub:
		if isTensorSubtypeKind(left) && isTensorSubtypeKind(right) {
			val, err := tensorAdditive(op)("TensorCalculations::'"+op.String()+"'", ctx, args)
			return val, true, err
		}
	case ast.OpMul:
		switch {
		case lt && right.Kind == ValQuantity:
			val, err := scalarQuantityTensorMult(1, 0)("TensorCalculations::TensorScalarQuantityMult", ctx, args)
			return val, true, err
		case lt && right.Kind == ValConst:
			val, err := scalarTensorMult(1, 0)("TensorCalculations::TensorScalarMult", ctx, args)
			return val, true, err
		case rt && left.Kind == ValQuantity:
			val, err := scalarQuantityTensorMult(0, 1)("TensorCalculations::scalarQuantityTensorMult", ctx, args)
			return val, true, err
		case rt && left.Kind == ValConst:
			val, err := scalarTensorMult(0, 1)("TensorCalculations::scalarTensorMult", ctx, args)
			return val, true, err
		case lt && rt:
			return Value{}, true, fmt.Errorf("%w: TensorCalculations::tensorTensorMult: %s", ErrUnevaluableLibraryFunction, noContractionConvention(tensorTensorMultDecl))
		case lt && isVectorKind(right):
			return Value{}, true, fmt.Errorf("%w: TensorCalculations::tensorVectorMult: %s", ErrUnevaluableLibraryFunction, noContractionConvention(tensorVectorMultDecl))
		case rt && isVectorKind(left):
			return Value{}, true, fmt.Errorf("%w: TensorCalculations::vectorTensorMult: %s", ErrUnevaluableLibraryFunction, noContractionConvention(vectorTensorMultDecl))
		}
	}
	return Value{}, true, fmt.Errorf("%w: TensorCalculations declares no '%s' over %s and %s",
		ErrUnevaluableLibraryFunction, op.String(), describeOperand(left), describeOperand(right))
}
