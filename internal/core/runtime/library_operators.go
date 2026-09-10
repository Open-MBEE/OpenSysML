package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// operandDomain binds an argument to the parameter type one function library
// package declares for its operators: the value the parameter holds, or why
// the argument does not conform.
type operandDomain func(ctx *Context, name, param string, val Value) (Value, error)

// operatorForms lists the operator functions each package declares over `x`
// and `y`, with the domain its parameter types impose. `..` is a builtin.
var operatorForms = []struct {
	pkg    string
	ops    []string
	domain operandDomain
}{
	{"DataFunctions", []string{"+", "-", "*", "/", "%", "**", "^", "<", "<=", ">", ">=", "not", "xor", "|", "&"}, dataOperand},
	{"ScalarFunctions", []string{"+", "-", "*", "/", "%", "**", "^", "<", "<=", ">", ">=", "not", "xor", "|", "&"}, anyOperand},
	{"NumericalFunctions", []string{"+", "-", "*", "/", "%", "**", "^", "<", "<=", ">", ">="}, numericalOperand},
	{"RealFunctions", []string{"+", "-", "*", "/", "**", "^", "<", "<=", ">", ">="}, realOperand},
	{"RationalFunctions", []string{"+", "-", "*", "/", "**", "^", "<", "<=", ">", ">="}, rationalOperand},
	{"IntegerFunctions", []string{"+", "-", "*", "/", "%", "<", "<=", ">", ">="}, integerOperand},
	{"IntegerFunctions", []string{"**", "^"}, integerBaseNaturalExponent},
	{"NaturalFunctions", []string{"+", "*", "%", "<", "<=", ">", ">="}, naturalOperand},
	{"BooleanFunctions", []string{"not", "xor", "|", "&"}, booleanOperand},
}

// operatorKinds maps each operator function's name to the operator it applies.
var operatorKinds = map[string]ast.OperatorKind{
	"+": ast.OpAdd, "-": ast.OpSub, "*": ast.OpMul, "/": ast.OpDiv, "%": ast.OpMod,
	"**": ast.OpPow, "^": ast.OpPow,
	"<": ast.OpLt, "<=": ast.OpLe, ">": ast.OpGt, ">=": ast.OpGe,
	"not": ast.OpNot, "xor": ast.OpXor, "|": ast.OpOr, "&": ast.OpAnd,
}

// equalityForms are the `'=='` declarations, each over two [0..1] operands
// of the package's type; an omitted operand is outside every domain check.
var equalityForms = map[string]operandDomain{
	"BaseFunctions::==":     anyOperand,
	"DataFunctions::==":     dataOperand,
	"BooleanFunctions::==":  booleanOperand,
	"IntegerFunctions::==":  integerOperand,
	"NaturalFunctions::==":  naturalOperand,
	"RationalFunctions::==": rationalOperand,
	"RealFunctions::==":     realOperand,
}

// registerOperatorFunctions registers the explicit function form of every
// operator the library declares, each delegating to the operator's evaluator.
func registerOperatorFunctions() {
	for _, form := range operatorForms {
		for _, op := range form.ops {
			registerOperatorForm(form.pkg+"::"+op, op, form.domain)
		}
	}
	for fqn, domain := range equalityForms {
		registerValueFunction(fqn, []string{"x", "y"}, 0, equalityForm(ast.OpEq, domain))
	}
	registerValueFunction("NaturalFunctions::/", []string{"x", "y"}, 2, naturalDivision)
	registerValueFunction("BaseFunctions::!=", []string{"x", "y"}, 0, equalityForm(ast.OpNeq, anyOperand))
	registerValueFunction("BaseFunctions::===", []string{"x", "y"}, 0, identityForm(false, anyOperand))
	registerValueFunction("DataFunctions::===", []string{"x", "y"}, 0, identityForm(false, dataOperand))
	registerValueFunction("BaseFunctions::!==", []string{"x", "y"}, 0, identityForm(true, anyOperand))
}

// registerOperatorForm registers one operator function. `+` and `-` declare
// `y` [0..1], so given one argument they are the unary operators.
func registerOperatorForm(fqn, op string, domain operandDomain) {
	kind := operatorKinds[op]
	switch op {
	case "not":
		registerValueFunction(fqn, []string{"x"}, 1, unaryForm(kind, domain))
	case "+", "-":
		registerValueFunction(fqn, []string{"x", "y"}, 1, arithmeticForm(kind, domain))
	case "*", "/", "%", "**", "^":
		registerValueFunction(fqn, []string{"x", "y"}, 2, arithmeticForm(kind, domain))
	case "<", "<=", ">", ">=":
		registerValueFunction(fqn, []string{"x", "y"}, 2, comparisonForm(kind, domain))
	case "xor", "|", "&":
		registerValueFunction(fqn, []string{"x", "y"}, 2, booleanForm(kind, domain))
	}
}

// checkOperands binds each argument given, declared `[1]`, through the
// package's domain: the one value it holds, a sole element standing for itself.
func checkOperands(ctx *Context, name string, domain operandDomain, args []Value) ([]Value, error) {
	bound := make([]Value, len(args))
	for i, param := range []string{"x", "y"}[:len(args)] {
		val, err := soleValue(name, param, args[i])
		if err != nil {
			return nil, err
		}
		val, err = domain(ctx, name, param, val)
		if err != nil {
			return nil, err
		}
		bound[i] = val
	}
	return bound, nil
}

// soleValue is the one value an operand declared `[1]` holds: a sole element
// stands for itself, and no element or several violate the multiplicity.
func soleValue(name, param string, val Value) (Value, error) {
	elements := elementsOf(val)
	if len(elements) != 1 {
		return Value{}, fmt.Errorf("%w: function %s parameter %q holds %d values, exactly 1 required",
			ErrMultiplicityViolation, name, param, len(elements))
	}
	return elements[0], nil
}

// singleOperands binds `x` and `y`, each declared `[0..1]`: an omitted one is
// null and outside every domain check, a sole element stands for itself, and
// several elements violate the multiplicity.
func singleOperands(ctx *Context, name string, domain operandDomain, args []Value) ([]Value, error) {
	bound := make([]Value, 2)
	for i, param := range []string{"x", "y"} {
		elements := elementsOf(args[i])
		switch len(elements) {
		case 0:
			bound[i] = nullValue()
			continue
		case 1:
		default:
			return nil, fmt.Errorf("%w: function %s parameter %q holds %d values, at most 1 allowed",
				ErrMultiplicityViolation, name, param, len(elements))
		}
		val, err := domain(ctx, name, param, elements[0])
		if err != nil {
			return nil, err
		}
		bound[i] = val
	}
	return bound, nil
}

// arithmeticForm is `'+'`, `'-'`, `'*'`, `'/'`, `'%'`, `'**'` and `'^'` as
// functions; `'+'(x)` and `'-'(x)` are the unary sign operators.
func arithmeticForm(op ast.OperatorKind, domain operandDomain) libraryApply {
	return func(name string, ctx *Context, args []Value) (Value, error) {
		if len(args) == 2 && argumentOmitted(args[1]) {
			args = args[:1]
		}
		args, err := checkOperands(ctx, name, domain, args)
		if err != nil {
			return Value{}, err
		}
		if len(args) == 1 {
			unary := ast.OpPos
			if op == ast.OpSub {
				unary = ast.OpNeg
			}
			val, err := unaryValue(unary, args[0])
			return operatorResult(name, val, err)
		}
		val, err := ctx.arithmeticValues(op, args[0], args[1], source.Span{})
		return operatorResult(name, val, err)
	}
}

// naturalDivision is NaturalFunctions::'/', declared to return a Natural: the
// exact quotient when y divides x, and no value otherwise, since truncating
// would compute something the declaration does not promise.
func naturalDivision(name string, ctx *Context, args []Value) (Value, error) {
	args, err := checkOperands(ctx, name, naturalOperand, args)
	if err != nil {
		return Value{}, err
	}
	x, y := args[0].Const.Int, args[1].Const.Int
	if y == 0 {
		return Value{}, fmt.Errorf("%w: function %s: %d / 0", ErrDivisionByZero, name, x)
	}
	if x%y != 0 {
		return Value{}, fmt.Errorf(
			"%w: function %s has no Natural result for %d / %d; the quotient is %s",
			semantics.ErrArithmeticDomain, name, x, y, semantics.FormatReal(float64(x)/float64(y)),
		)
	}
	return integerValue(x / y), nil
}

// comparisonForm is `'<'`, `'<='`, `'>'` and `'>='` as functions.
func comparisonForm(op ast.OperatorKind, domain operandDomain) libraryApply {
	return func(name string, ctx *Context, args []Value) (Value, error) {
		args, err := checkOperands(ctx, name, domain, args)
		if err != nil {
			return Value{}, err
		}
		val, err := comparisonValues(op, args[0], args[1], source.Span{})
		return operatorResult(name, val, err)
	}
}

// booleanForm is `'xor'`, `'|'` and `'&'` as functions; both operands are
// given, so nothing is short-circuited.
func booleanForm(op ast.OperatorKind, domain operandDomain) libraryApply {
	return func(name string, ctx *Context, args []Value) (Value, error) {
		args, err := checkOperands(ctx, name, domain, args)
		if err != nil {
			return Value{}, err
		}
		l, err := boolOperand(fmt.Sprintf("function %s parameter %q", name, "x"), args[0])
		if err != nil {
			return Value{}, err
		}
		r, err := boolOperand(fmt.Sprintf("function %s parameter %q", name, "y"), args[1])
		if err != nil {
			return Value{}, err
		}
		return combineBooleans(op, l, r)
	}
}

// unaryForm is `'not'` as a function.
func unaryForm(op ast.OperatorKind, domain operandDomain) libraryApply {
	return func(name string, ctx *Context, args []Value) (Value, error) {
		args, err := checkOperands(ctx, name, domain, args)
		if err != nil {
			return Value{}, err
		}
		val, err := unaryValue(op, args[0])
		return operatorResult(name, val, err)
	}
}

// equalityForm is `'=='` or `'!='` as a function; an omitted operand is null,
// so two omitted operands are equal and an omitted one equals no value.
func equalityForm(op ast.OperatorKind, domain operandDomain) libraryApply {
	return func(name string, ctx *Context, args []Value) (Value, error) {
		args, err := singleOperands(ctx, name, domain, args)
		if err != nil {
			return Value{}, err
		}
		val, err := ctx.equalityValues(op, args[0], args[1])
		return operatorResult(name, val, err)
	}
}

// identityForm is `'==='` (negated=false) or `'!=='` as a function over the
// package's operand domain.
func identityForm(negated bool, domain operandDomain) libraryApply {
	return func(name string, ctx *Context, args []Value) (Value, error) {
		args, err := singleOperands(ctx, name, domain, args)
		if err != nil {
			return Value{}, err
		}
		return boolValue(valueIdentical(args[0], args[1]) != negated), nil
	}
}

// registerGenericExtrema registers the `max` and `min` DataFunctions and
// ScalarFunctions declare over every ordered value.
func registerGenericExtrema() {
	for _, pkg := range []string{"DataFunctions", "ScalarFunctions"} {
		registerValueFunction(pkg+"::max", []string{"x", "y"}, 2, genericExtremum(true))
		registerValueFunction(pkg+"::min", []string{"x", "y"}, 2, genericExtremum(false))
	}
}

// genericExtremum picks the larger (or smaller) of two values the way the
// ordering operators compare them: numbers keep their kind as the numeric
// `max`/`min` do, and strings and quantities answer with the operand chosen.
// A kind the library declares no ordering for is refused.
func genericExtremum(larger bool) libraryApply {
	extremum := numericScalars([]string{"x", "y"}, numericExtremum(larger))
	return func(name string, ctx *Context, args []Value) (Value, error) {
		args, err := checkOperands(ctx, name, anyOperand, args)
		if err != nil {
			return Value{}, err
		}
		x, y := args[0], args[1]
		switch {
		case x.Kind == ValConst && x.Const.IsNumeric() && y.Kind == ValConst && y.Const.IsNumeric():
			return extremum(name, ctx, args)
		case x.Kind == ValString && y.Kind == ValString, x.Kind == ValQuantity && y.Kind == ValQuantity:
			less, err := comparisonValues(ast.OpLt, x, y, source.Span{})
			if err != nil {
				return Value{}, fmt.Errorf("function %s: %w", name, err)
			}
			if less.Const.Bool == larger {
				return y, nil
			}
			return x, nil
		}
		return Value{}, fmt.Errorf(
			"%w: function %s orders numbers, strings and quantities, not %s and %s",
			ErrTypeMismatch, name, describeOperand(x), describeOperand(y),
		)
	}
}

// operatorResult names the function an operator error was raised in.
func operatorResult(name string, val Value, err error) (Value, error) {
	if err != nil {
		return Value{}, fmt.Errorf("function %s: %w", name, err)
	}
	return val, nil
}

// anyOperand is the domain of BaseFunctions and ScalarFunctions, whose
// operators the evaluator defines for whichever values it can.
func anyOperand(_ *Context, _, _ string, val Value) (Value, error) { return val, nil }

// dataOperand admits a DataValue: a scalar, a collection of them, or an object
// whose type conforms to Base::DataValue — not a part, item or other occurrence.
func dataOperand(ctx *Context, name, param string, val Value) (Value, error) {
	switch val.Kind {
	case ValConst, ValString, ValQuantity, ValEnumLiteral, ValComplex, ValVector, ValVectorQuantity, ValTensorQuantity, ValMeasurementRef,
		ValCoordinateFrame, ValCoordinateTransformation:
		return val, nil
	case ValSequence, ValSet:
		for _, element := range elementsOf(val) {
			if _, err := dataOperand(ctx, name, param, element); err != nil {
				return Value{}, err
			}
		}
		return val, nil
	case ValArray:
		for _, element := range val.Array().Elements {
			if _, err := dataOperand(ctx, name, param, element); err != nil {
				return Value{}, err
			}
		}
		return val, nil
	case ValInstance, ValVariant:
		if ctx.isDataValue(val) {
			return val, nil
		}
	}
	return Value{}, operandMismatch(name, param, "a DataValue", val)
}

// isDataValue reports whether a type of an object conforms to Base::DataValue.
func (ctx *Context) isDataValue(val Value) bool {
	direct, err := ctx.directValueTypes(nil, val)
	if err != nil {
		return false
	}
	dataValue := ctx.librarySymbol("Base::DataValue")
	if dataValue == nil {
		return false
	}
	for _, typ := range direct {
		if ctx.model.Conforms(typ, dataValue) {
			return true
		}
	}
	return false
}

// numericalOperand admits the NumericalValues: Integers, Reals and Complexes.
func numericalOperand(_ *Context, name, param string, val Value) (Value, error) {
	if val.Kind == ValComplex || (val.Kind == ValConst && val.Const.IsNumeric()) {
		return val, nil
	}
	return Value{}, operandMismatch(name, param, "a numeric", val)
}

// realOperand binds an Integer or a Real to a Real parameter, which holds the
// Real the argument equals: RealFunctions compute over Reals and answer one.
func realOperand(_ *Context, name, param string, val Value) (Value, error) {
	if val.Kind == ValConst && val.Const.IsNumeric() {
		return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: asReal(val.Const)}}, nil
	}
	return Value{}, operandMismatch(name, param, "a Real", val)
}

// rationalOperand admits an Integer or a Real as a Rational; an Integer keeps
// its kind, as RationalFunctions::abs/max/min keep it.
func rationalOperand(_ *Context, name, param string, val Value) (Value, error) {
	if val.Kind == ValConst && val.Const.IsNumeric() {
		return val, nil
	}
	return Value{}, operandMismatch(name, param, "a Rational", val)
}

// integerOperand admits an Integer only: a Real does not conform to Integer.
func integerOperand(_ *Context, name, param string, val Value) (Value, error) {
	if val.Kind == ValConst && val.Const.Kind == semantics.ValInt {
		return val, nil
	}
	return Value{}, operandMismatch(name, param, "an Integer", val)
}

// naturalOperand admits a non-negative Integer.
func naturalOperand(ctx *Context, name, param string, val Value) (Value, error) {
	if _, err := integerOperand(ctx, name, param, val); err != nil {
		return Value{}, err
	}
	if val.Const.Int < 0 {
		return Value{}, fmt.Errorf("%w: function %s parameter %q requires a Natural value, got %d", ErrTypeMismatch, name, param, val.Const.Int)
	}
	return val, nil
}

// integerBaseNaturalExponent is IntegerFunctions::'**' and '^': `x` an Integer
// and `y` a Natural, so the result is an Integer.
func integerBaseNaturalExponent(ctx *Context, name, param string, val Value) (Value, error) {
	if param == "y" {
		return naturalOperand(ctx, name, param, val)
	}
	return integerOperand(ctx, name, param, val)
}

// booleanOperand admits a Boolean.
func booleanOperand(_ *Context, name, param string, val Value) (Value, error) {
	if val.Kind == ValConst && val.Const.Kind == semantics.ValBool {
		return val, nil
	}
	return Value{}, operandMismatch(name, param, "a Boolean", val)
}

func operandMismatch(name, param, want string, val Value) error {
	return fmt.Errorf("%w: function %s parameter %q requires %s value, got %s", ErrTypeMismatch, name, param, want, describeOperand(val))
}
