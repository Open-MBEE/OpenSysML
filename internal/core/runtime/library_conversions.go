package runtime

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// ErrInvalidNotation reports a String a conversion function cannot read as a
// value of the type it converts to.
var ErrInvalidNotation = errors.New("string is not a valid notation")

// registerConversionFunctions registers the Kernel Function Library's
// conversions between String, Boolean and the numeric scalar types, plus the
// Rational functions the float64 representation can answer.
func registerConversionFunctions() {
	registerValueFunction("BaseFunctions::ToString", []string{"x"}, 0, anythingToString)
	registerValueFunction("BooleanFunctions::ToString", []string{"x"}, 1, booleanToString)
	registerValueFunction("IntegerFunctions::ToString", []string{"x"}, 1, integerToString)
	registerValueFunction("NaturalFunctions::ToString", []string{"x"}, 1, naturalToString)
	registerValueFunction("RationalFunctions::ToString", []string{"x"}, 1, numberToString)
	registerValueFunction("RealFunctions::ToString", []string{"x"}, 1, numberToString)

	registerValueFunction("BooleanFunctions::ToBoolean", []string{"x"}, 1, stringToBoolean)
	registerValueFunction("IntegerFunctions::ToInteger", []string{"x"}, 1, stringToInteger)
	registerValueFunction("NaturalFunctions::ToNatural", []string{"x"}, 1, stringToNatural)
	registerValueFunction("RationalFunctions::ToRational", []string{"x"}, 1, stringToRational)
	registerValueFunction("RealFunctions::ToReal", []string{"x"}, 1, stringToReal)

	registerValueFunction("IntegerFunctions::ToNatural", []string{"x"}, 1, integerToNatural)
	registerValueFunction("RationalFunctions::ToInteger", []string{"x"}, 1, numberToInteger)
	registerValueFunction("RealFunctions::ToInteger", []string{"x"}, 1, numberToInteger)
	registerValueFunction("RealFunctions::ToRational", []string{"x"}, 1, realItself)

	// A Real is a Complex with a zero imaginary part, so the Complex functions
	// answer `re` and `im`; the library gives `arg` the body `0.0`.
	registerValueFunction("RealFunctions::re", []string{"x"}, 1, realItself)
	registerValueFunction("RealFunctions::im", []string{"x"}, 1, realZero)
	registerValueFunction("RealFunctions::arg", []string{"x"}, 1, realZero)

	registerLibraryFunction("RationalFunctions::floor", []string{"x"}, floorToInteger)
	registerLibraryFunction("RationalFunctions::round", []string{"x"}, roundToInteger)
	registerLibraryFunction("RationalFunctions::gcd", []string{"x", "y"}, rationalGCD, wholeDomain, wholeDomain)
	registerLibraryFunction("RationalFunctions::rat", []string{"numer", "denum"}, integersToRational, integerDomain, denominatorDomain)
	registerLibraryFunction("RationalFunctions::numer", []string{"rat"}, rationalNumerator)
	registerLibraryFunction("RationalFunctions::denom", []string{"rat"}, rationalDenominator)
}

// anythingToString is BaseFunctions::ToString, the notation x is written with in
// a model: a literal, an enumeration literal by name, a quantity with its unit.
// x is Anything[0..1], so an omitted x is the null value and reads `null`.
func anythingToString(name string, _ *Context, args []Value) (Value, error) {
	elements := elementsOf(args[0])
	if len(elements) > 1 {
		return Value{}, fmt.Errorf(
			"%w: function %s parameter %q is Anything[0..1], got %d values",
			ErrMultiplicityViolation, name, "x", len(elements),
		)
	}
	if len(elements) == 0 {
		return NewStringValue("null"), nil
	}
	x := elements[0]
	switch x.Kind {
	case ValString:
		return x, nil
	case ValConst:
		if x.Const.Kind == semantics.ValInfinity {
			return NewStringValue("*"), nil
		}
		return NewStringValue(semantics.FormatConst(x.Const)), nil
	case ValQuantity, ValEnumLiteral, ValMeasurementRef, ValCoordinateFrame, ValCoordinateTransformation:
		return NewStringValue(FormatValue(x)), nil
	case ValComplex:
		return Value{}, fmt.Errorf("%w: %s: no string notation for a Complex value is defined", ErrUnevaluableLibraryFunction, name)
	case ValArray, ValVector, ValVectorQuantity, ValTensorQuantity:
		return Value{}, fmt.Errorf("%w: %s: no string notation for %s is defined", ErrUnevaluableLibraryFunction, name, describeOperand(x))
	}
	return Value{}, fmt.Errorf(
		"%w: function %s parameter %q has no String notation for %s",
		ErrTypeMismatch, name, "x", describeOperand(x),
	)
}

// booleanToString is BooleanFunctions::ToString: `true` or `false`.
func booleanToString(name string, _ *Context, args []Value) (Value, error) {
	b, err := booleanArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	return NewStringValue(strconv.FormatBool(b)), nil
}

// integerToString is IntegerFunctions::ToString, the decimal digits of x.
func integerToString(name string, _ *Context, args []Value) (Value, error) {
	x, err := integerArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	return NewStringValue(strconv.FormatInt(x, 10)), nil
}

// naturalToString is NaturalFunctions::ToString over a Natural argument.
func naturalToString(name string, _ *Context, args []Value) (Value, error) {
	x, err := naturalArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	return NewStringValue(strconv.FormatInt(x, 10)), nil
}

// numberToString is RealFunctions::ToString and RationalFunctions::ToString:
// the shortest decimal that reads back as the same value, as the REPL prints
// it, so ToReal(ToString(x)) == x. An Integer argument keeps its digits.
func numberToString(name string, _ *Context, args []Value) (Value, error) {
	x, err := numericArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	return NewStringValue(semantics.FormatConst(x)), nil
}

// stringToBoolean is BooleanFunctions::ToBoolean over the two Boolean literals.
func stringToBoolean(name string, _ *Context, args []Value) (Value, error) {
	s, err := stringArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	switch s {
	case "true":
		return boolValue(true), nil
	case "false":
		return boolValue(false), nil
	}
	return Value{}, invalidNotation(name, s, "Boolean")
}

// stringToInteger is IntegerFunctions::ToInteger over decimal digits with an
// optional sign; a value outside the Integer range overflows.
func stringToInteger(name string, _ *Context, args []Value) (Value, error) {
	s, err := stringArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	x, err := parseInteger(name, s, "Integer")
	if err != nil {
		return Value{}, err
	}
	return integerValue(x), nil
}

// stringToNatural is NaturalFunctions::ToNatural: an Integer notation whose
// value is not negative.
func stringToNatural(name string, _ *Context, args []Value) (Value, error) {
	s, err := stringArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	x, err := parseInteger(name, s, "Natural")
	if err != nil {
		return Value{}, err
	}
	return naturalValue(name, x)
}

// stringToReal is RealFunctions::ToReal over decimal notation, with or without
// a fraction or exponent; the result is a finite Real.
func stringToReal(name string, _ *Context, args []Value) (Value, error) {
	s, err := stringArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	return parseReal(name, s, "Real")
}

// stringToRational is RationalFunctions::ToRational, which reads the same
// decimal notation as ToReal since a Rational is a float64 here.
func stringToRational(name string, _ *Context, args []Value) (Value, error) {
	s, err := stringArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	return parseReal(name, s, "Rational")
}

// integerToNatural is IntegerFunctions::ToNatural; a negative Integer is no
// Natural and is a domain error rather than a wrapped or clamped value.
func integerToNatural(name string, _ *Context, args []Value) (Value, error) {
	x, err := integerArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	return naturalValue(name, x)
}

// numberToInteger is RealFunctions::ToInteger and RationalFunctions::ToInteger.
// The library declares floor and round separately and gives ToInteger no body,
// so it converts the way a numeric narrowing does: toward zero.
func numberToInteger(name string, _ *Context, args []Value) (Value, error) {
	x, err := numericArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	if x.Kind == semantics.ValInt {
		return Value{Kind: ValConst, Const: x}, nil
	}
	result, err := integerResult(math.Trunc(x.Real))
	if err != nil {
		return Value{}, fmt.Errorf("function %s: %w", name, err)
	}
	return Value{Kind: ValConst, Const: result}, nil
}

// realItself answers the Real x is bound as, an Integer argument widened: it is
// RealFunctions::ToRational (a Rational is held as a float64) and RealFunctions::re
// (a Real is its own real part).
func realItself(name string, _ *Context, args []Value) (Value, error) {
	x, err := numericArg(name, "x", args[0])
	if err != nil {
		return Value{}, err
	}
	return checkedReal(asReal(x))
}

// realZero answers 0.0 for every Real: RealFunctions::im and RealFunctions::arg,
// whose vendored bodies are both `0.0`.
func realZero(name string, _ *Context, args []Value) (Value, error) {
	if _, err := numericArg(name, "x", args[0]); err != nil {
		return Value{}, err
	}
	return checkedReal(0)
}

// rationalGCD is RationalFunctions::gcd, the greatest common divisor of two
// whole values as a non-negative Integer; gcd(0, 0) is 0. A Rational with a
// fractional part has no Integer divisor to answer with.
func rationalGCD(args []semantics.Value) (semantics.Value, error) {
	x, _ := wholeInteger(args[0])
	y, _ := wholeInteger(args[1])
	// Exact over any whole operand, so MinInt64 and whole Reals past the Integer
	// range are divisors like any other; only the result must be an Integer.
	gcd := new(big.Int).GCD(nil, nil, x.Abs(x), y.Abs(y))
	if !gcd.IsInt64() {
		return semantics.Value{}, fmt.Errorf("%w: gcd(%s, %s) exceeds the Integer range",
			semantics.ErrArithmeticOverflow, semantics.FormatConst(args[0]), semantics.FormatConst(args[1]))
	}
	return semantics.Value{Kind: semantics.ValInt, Int: gcd.Int64()}, nil
}

// wholeDomain is the domain of gcd's parameters: any Integer, or a Real with no
// fractional part.
func wholeDomain(v semantics.Value) error {
	if _, ok := wholeInteger(v); !ok {
		return fmt.Errorf("%w: gcd is defined over whole values, got %s", semantics.ErrArithmeticDomain, semantics.FormatConst(v))
	}
	return nil
}

// denominatorDomain is the domain of rat's denum: an Integer other than zero.
func denominatorDomain(v semantics.Value) error {
	if err := integerDomain(v); err != nil {
		return err
	}
	if v.Int == 0 {
		return ErrDivisionByZero
	}
	return nil
}

// integersToRational is RationalFunctions::rat: numer/denum rounded once to the
// float64 a Rational is here, exactly as `/` computes it.
func integersToRational(args []semantics.Value) (semantics.Value, error) {
	q, _ := semantics.IntQuotient(args[0].Int, args[1].Int)
	return semantics.Value{Kind: semantics.ValReal, Real: q}, nil
}

// rationalNumerator is RationalFunctions::numer, the numerator of the ratio in
// lowest terms the Rational holds (docs/project/exact-rational-evaluation.md).
func rationalNumerator(args []semantics.Value) (semantics.Value, error) {
	return exactRationalTerm("numer", args[0], (*big.Rat).Num)
}

// rationalDenominator is RationalFunctions::denom, the positive denominator of
// the ratio in lowest terms the Rational holds.
func rationalDenominator(args []semantics.Value) (semantics.Value, error) {
	return exactRationalTerm("denom", args[0], (*big.Rat).Denom)
}

// exactRationalTerm reads one term of the exact ratio x holds: an Integer is
// itself over 1, a float64 the dyadic ratio its bits denote; overflow is reported.
func exactRationalTerm(fn string, x semantics.Value, term func(*big.Rat) *big.Int) (semantics.Value, error) {
	var ratio *big.Rat
	if x.Kind == semantics.ValInt {
		ratio = new(big.Rat).SetInt64(x.Int)
	} else if ratio = new(big.Rat).SetFloat64(x.Real); ratio == nil {
		return semantics.Value{}, fmt.Errorf("%w: %s(%s) has no finite ratio", semantics.ErrArithmeticDomain, fn, semantics.FormatConst(x))
	}
	result := term(ratio)
	if !result.IsInt64() {
		return semantics.Value{}, fmt.Errorf("%w: %s(%s) is %s, which exceeds the Integer range",
			semantics.ErrArithmeticOverflow, fn, semantics.FormatConst(x), result)
	}
	return semantics.Value{Kind: semantics.ValInt, Int: result.Int64()}, nil
}

// wholeInteger is the exact integer a whole numeric value holds: any Integer,
// or a finite Real with no fractional part, however large.
func wholeInteger(v semantics.Value) (*big.Int, bool) {
	switch v.Kind {
	case semantics.ValInt:
		return big.NewInt(v.Int), true
	case semantics.ValReal:
		if math.IsInf(v.Real, 0) || math.IsNaN(v.Real) || v.Real != math.Trunc(v.Real) {
			return nil, false
		}
		n, _ := new(big.Float).SetFloat64(v.Real).Int(nil)
		return n, true
	}
	return nil, false
}

// parseInteger reads decimal Integer notation, reporting anything else as an
// invalid notation for the named type and an out-of-range value as overflow.
func parseInteger(name, s, typeName string) (int64, error) {
	x, err := strconv.ParseInt(s, 10, 64)
	if err == nil {
		return x, nil
	}
	if errors.Is(err, strconv.ErrRange) {
		return 0, fmt.Errorf("%w: function %s: %s exceeds the Integer range", semantics.ErrArithmeticOverflow, name, s)
	}
	return 0, invalidNotation(name, s, typeName)
}

// parseReal reads decimal Real notation into a finite Real, reporting any
// other notation as invalid for the named type and a magnitude float64
// cannot hold as overflow.
func parseReal(name, s, typeName string) (Value, error) {
	x, err := semantics.ParseReal(s)
	if errors.Is(err, semantics.ErrRealNotation) {
		return Value{}, invalidNotation(name, s, typeName)
	}
	if err != nil {
		return Value{}, fmt.Errorf("%w: function %s: %s is outside the Real range", err, name, s)
	}
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: x}}, nil
}

// invalidNotation is the error for a String that is no notation of a type.
func invalidNotation(name, s, typeName string) error {
	return fmt.Errorf("%w: function %s: %s is not a %s", ErrInvalidNotation, name, strconv.Quote(s), typeName)
}

// naturalValue wraps a non-negative Integer as a Natural.
func naturalValue(name string, x int64) (Value, error) {
	if x < 0 {
		return Value{}, fmt.Errorf("%w: function %s: no Natural equals %d", semantics.ErrArithmeticDomain, name, x)
	}
	return integerValue(x), nil
}

// booleanArg reads a Boolean parameter.
func booleanArg(name, param string, val Value) (bool, error) {
	val = soleElement(val)
	if val.Kind != ValConst || val.Const.Kind != semantics.ValBool {
		return false, fmt.Errorf(
			"%w: function %s parameter %q requires a Boolean value, got %s",
			ErrTypeMismatch, name, param, describeOperand(val),
		)
	}
	return val.Const.Bool, nil
}

// numericArg reads an Integer or Real parameter.
func numericArg(name, param string, val Value) (semantics.Value, error) {
	val = soleElement(val)
	if val.Kind != ValConst || !val.Const.IsNumeric() {
		return semantics.Value{}, fmt.Errorf(
			"%w: function %s parameter %q requires a numeric value, got %s",
			ErrTypeMismatch, name, param, describeOperand(val),
		)
	}
	return val.Const, nil
}

// integerArg reads an Integer parameter; a Real does not conform to it.
func integerArg(name, param string, val Value) (int64, error) {
	val = soleElement(val)
	if val.Kind != ValConst || val.Const.Kind != semantics.ValInt {
		return 0, fmt.Errorf(
			"%w: function %s parameter %q requires an Integer value, got %s",
			ErrTypeMismatch, name, param, describeOperand(val),
		)
	}
	return val.Const.Int, nil
}

// naturalArg reads a Natural parameter: an Integer that is not negative.
func naturalArg(name, param string, val Value) (int64, error) {
	x, err := integerArg(name, param, val)
	if err != nil {
		return 0, err
	}
	if x < 0 {
		return 0, fmt.Errorf(
			"%w: function %s parameter %q requires a Natural value, got %d",
			ErrTypeMismatch, name, param, x,
		)
	}
	return x, nil
}
