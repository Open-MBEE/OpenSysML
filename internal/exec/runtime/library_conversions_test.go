package runtime

import (
	"errors"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

func constBool(b bool) Value {
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: b}}
}

// The conversions compute the value their declaration promises, keeping the
// kind the declared result type names.
func TestConversionFunctionValues(t *testing.T) {
	cases := []struct {
		fn   string
		args []Value
		want Value
	}{
		{"BaseFunctions::ToString", []Value{constInt(42)}, NewStringValue("42")},
		{"BaseFunctions::ToString", []Value{constReal(2.5)}, NewStringValue("2.5")},
		{"BaseFunctions::ToString", []Value{constReal(1)}, NewStringValue("1.0")},
		{"BaseFunctions::ToString", []Value{constReal(math.Nextafter(0.3, 1))}, NewStringValue("0.30000000000000004")},
		{"BaseFunctions::ToString", []Value{constReal(1e21)}, NewStringValue("1e+21")},
		{"BaseFunctions::ToString", []Value{constBool(true)}, NewStringValue("true")},
		{"BaseFunctions::ToString", []Value{NewStringValue("as is")}, NewStringValue("as is")},
		{"BaseFunctions::ToString", []Value{nullValue()}, NewStringValue("null")},
		{"BaseFunctions::ToString", []Value{NewSequenceValue(NewSequence())}, NewStringValue("null")},
		{"BaseFunctions::ToString", []Value{constSequence(3)}, NewStringValue("3")},
		{"BaseFunctions::ToString", []Value{Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInfinity}}}, NewStringValue("*")},
		{"BaseFunctions::ToString", []Value{NewQuantityValue(&Quantity{
			Num: semantics.Value{Kind: semantics.ValReal, Real: 1.5}, Unit: Unit{Text: "m", Term: semantics.UnitTerm{Scale: semantics.UnitScale(1)}}})}, NewStringValue("1.5 [m]")},
		{"BooleanFunctions::ToString", []Value{constBool(false)}, NewStringValue("false")},
		{"IntegerFunctions::ToString", []Value{constInt(-7)}, NewStringValue("-7")},
		{"NaturalFunctions::ToString", []Value{constInt(7)}, NewStringValue("7")},
		{"RationalFunctions::ToString", []Value{constReal(0.5)}, NewStringValue("0.5")},
		{"RealFunctions::ToString", []Value{constReal(-1.5)}, NewStringValue("-1.5")},
		{"RealFunctions::ToString", []Value{constInt(3)}, NewStringValue("3")},

		{"BooleanFunctions::ToBoolean", []Value{NewStringValue("true")}, constBool(true)},
		{"BooleanFunctions::ToBoolean", []Value{NewStringValue("false")}, constBool(false)},
		{"IntegerFunctions::ToInteger", []Value{NewStringValue("7")}, constInt(7)},
		{"IntegerFunctions::ToInteger", []Value{NewStringValue("-9223372036854775808")}, constInt(math.MinInt64)},
		{"NaturalFunctions::ToNatural", []Value{NewStringValue("0")}, constInt(0)},
		{"RealFunctions::ToReal", []Value{NewStringValue("1.5")}, constReal(1.5)},
		{"RealFunctions::ToReal", []Value{NewStringValue("-2")}, constReal(-2)},
		{"RealFunctions::ToReal", []Value{NewStringValue("1e21")}, constReal(1e21)},
		{"RealFunctions::ToReal", []Value{NewStringValue(".5")}, constReal(0.5)},
		{"RealFunctions::ToReal", []Value{NewStringValue("0e-400")}, constReal(0)},
		{"RealFunctions::ToReal", []Value{NewStringValue("-0.000")}, constReal(0)},
		{"RealFunctions::ToReal", []Value{NewStringValue("4.9e-324")}, constReal(math.SmallestNonzeroFloat64)},
		{"RationalFunctions::ToRational", []Value{NewStringValue("0.25")}, constReal(0.25)},

		{"IntegerFunctions::ToNatural", []Value{constInt(5)}, constInt(5)},
		{"RealFunctions::ToInteger", []Value{constReal(2.7)}, constInt(2)},
		{"RealFunctions::ToInteger", []Value{constReal(-2.7)}, constInt(-2)},
		{"RealFunctions::ToInteger", []Value{constInt(4)}, constInt(4)},
		{"RationalFunctions::ToInteger", []Value{constReal(0.5)}, constInt(0)},
		{"RealFunctions::ToRational", []Value{constReal(0.75)}, constReal(0.75)},
		{"RealFunctions::ToRational", []Value{constInt(2)}, constReal(2)},

		{"RealFunctions::re", []Value{constReal(2.5)}, constReal(2.5)},
		{"RealFunctions::re", []Value{constInt(2)}, constReal(2)},
		{"RealFunctions::im", []Value{constReal(2.5)}, constReal(0)},
		{"RealFunctions::arg", []Value{constReal(-2.5)}, constReal(0)},

		{"RationalFunctions::floor", []Value{constReal(-0.5)}, constInt(-1)},
		{"RationalFunctions::round", []Value{constReal(2.5)}, constInt(3)},
		{"RationalFunctions::gcd", []Value{constInt(12), constInt(18)}, constInt(6)},
		{"RationalFunctions::gcd", []Value{constInt(-12), constInt(18)}, constInt(6)},
		{"RationalFunctions::gcd", []Value{constInt(0), constInt(0)}, constInt(0)},
		{"RationalFunctions::gcd", []Value{constInt(0), constInt(-5)}, constInt(5)},
		{"RationalFunctions::gcd", []Value{constReal(12), constInt(8)}, constInt(4)},
		{"RationalFunctions::gcd", []Value{constInt(math.MinInt64), constInt(2)}, constInt(2)},
		{"RationalFunctions::gcd", []Value{constInt(6), constInt(math.MinInt64)}, constInt(2)},
		{"RationalFunctions::gcd", []Value{constInt(math.MinInt64), constInt(3)}, constInt(1)},
		{"RationalFunctions::gcd", []Value{constInt(math.MinInt64), constInt(1 << 62)}, constInt(1 << 62)},
		{"RationalFunctions::gcd", []Value{constInt(math.MinInt64), constInt(math.MaxInt64)}, constInt(1)},
		{"RationalFunctions::gcd", []Value{constInt(math.MinInt64), constInt(math.MinInt64 + 1)}, constInt(1)},
		{"RationalFunctions::gcd", []Value{constReal(math.Ldexp(1, 63)), constInt(2)}, constInt(2)},
		{"RationalFunctions::gcd", []Value{constInt(6), constReal(math.Ldexp(1, 64))}, constInt(2)},
		{"RationalFunctions::gcd", []Value{constReal(1e20), constReal(6e18)}, constInt(2e18)},
		{"RationalFunctions::gcd", []Value{constReal(-1e300), constInt(1 << 62)}, constInt(1 << 62)},

		{"RationalFunctions::rat", []Value{constInt(1), constInt(3)}, constReal(1.0 / 3.0)},
		{"RationalFunctions::rat", []Value{constInt(6), constInt(4)}, constReal(1.5)},
		{"RationalFunctions::rat", []Value{constInt(-3), constInt(4)}, constReal(-0.75)},
		{"RationalFunctions::rat", []Value{constInt(3), constInt(-4)}, constReal(-0.75)},
		{"RationalFunctions::rat", []Value{constInt(0), constInt(1)}, constReal(0)},
		{"RationalFunctions::rat", []Value{constInt(4), constInt(2)}, constReal(2)},
		{"RationalFunctions::rat", []Value{constInt(3602879701896397), constInt(1 << 55)}, constReal(0.1)},
		{"RationalFunctions::rat", []Value{constInt(math.MinInt64), constInt(math.MinInt64)}, constReal(1)},
		{"RationalFunctions::rat", []Value{constInt(math.MaxInt64), constInt(1)}, constReal(math.Ldexp(1, 63))},
		{"RationalFunctions::numer", []Value{constReal(0.75)}, constInt(3)},
		{"RationalFunctions::denom", []Value{constReal(0.75)}, constInt(4)},
		{"RationalFunctions::numer", []Value{constReal(-0.75)}, constInt(-3)},
		{"RationalFunctions::denom", []Value{constReal(-0.75)}, constInt(4)},
		{"RationalFunctions::numer", []Value{constReal(1.5)}, constInt(3)},
		{"RationalFunctions::denom", []Value{constReal(1.5)}, constInt(2)},
		{"RationalFunctions::numer", []Value{constReal(0)}, constInt(0)},
		{"RationalFunctions::denom", []Value{constReal(0)}, constInt(1)},
		{"RationalFunctions::numer", []Value{constReal(math.Copysign(0, -1))}, constInt(0)},
		{"RationalFunctions::denom", []Value{constReal(math.Copysign(0, -1))}, constInt(1)},
		{"RationalFunctions::numer", []Value{constReal(2)}, constInt(2)},
		{"RationalFunctions::denom", []Value{constReal(2)}, constInt(1)},
		{"RationalFunctions::numer", []Value{constInt(2)}, constInt(2)},
		{"RationalFunctions::denom", []Value{constInt(2)}, constInt(1)},
		{"RationalFunctions::numer", []Value{constInt(-7)}, constInt(-7)},
		{"RationalFunctions::denom", []Value{constInt(-7)}, constInt(1)},
		{"RationalFunctions::numer", []Value{constInt(math.MinInt64)}, constInt(math.MinInt64)},
		{"RationalFunctions::numer", []Value{constReal(0.1)}, constInt(3602879701896397)},
		{"RationalFunctions::denom", []Value{constReal(0.1)}, constInt(1 << 55)},
		{"RationalFunctions::numer", []Value{constReal(1.0 / 3.0)}, constInt(6004799503160661)},
		{"RationalFunctions::denom", []Value{constReal(1.0 / 3.0)}, constInt(1 << 54)},
		{"RationalFunctions::numer", []Value{constReal(1e18)}, constInt(1e18)},
		{"RationalFunctions::denom", []Value{constReal(1e18)}, constInt(1)},
		{"RationalFunctions::denom", []Value{constReal(math.Ldexp(1, -62))}, constInt(1 << 62)},
		{"RationalFunctions::numer", []Value{constReal(math.Ldexp(1, -62))}, constInt(1)},
		{"RationalFunctions::numer", []Value{constReal(0.0001)}, constInt(7378697629483821)},
	}
	for _, tc := range cases {
		t.Run(tc.fn+"/"+FormatValue(tc.args[0]), func(t *testing.T) {
			got, err := applyLibrary(t, tc.fn, tc.args...)
			if err != nil {
				t.Fatalf("%s = error %v", tc.fn, err)
			}
			if !valueIdentical(got, tc.want) {
				t.Fatalf("%s = %s (%v), want %s (%v)", tc.fn, FormatValue(got), got.Kind, FormatValue(tc.want), tc.want.Kind)
			}
		})
	}
}

// rat(numer(x), denom(x)) is x for every finite Rational whose exact terms are
// Integers: the terms are the ratio x holds, and rat rounds that ratio to x.
func TestRationalTermsRoundTrip(t *testing.T) {
	for _, x := range []Value{
		constReal(0), constReal(math.Copysign(0, -1)), constReal(1), constReal(-1), constReal(2), constReal(-7),
		constReal(0.75), constReal(-0.75), constReal(1.5), constReal(0.1), constReal(-0.1), constReal(1.0 / 3.0),
		constReal(0.1 + 0.2), constReal(1e18), constReal(-9.2e18), constReal(math.Pi), constReal(math.Ldexp(1, -62)),
		constReal(math.Nextafter(1, 2)), constReal(123456789.125),
		constInt(0), constInt(1), constInt(-5), constInt(math.MaxInt64), constInt(math.MinInt64),
	} {
		numer, err := applyLibrary(t, "RationalFunctions::numer", x)
		if err != nil {
			t.Fatalf("numer(%s) = error %v", FormatValue(x), err)
		}
		denom, err := applyLibrary(t, "RationalFunctions::denom", x)
		if err != nil {
			t.Fatalf("denom(%s) = error %v", FormatValue(x), err)
		}
		if numer.Const.Kind != semantics.ValInt || denom.Const.Kind != semantics.ValInt || denom.Const.Int < 1 {
			t.Fatalf("numer(%s), denom(%s) = %s, %s, want Integers with a positive denominator", FormatValue(x), FormatValue(x), FormatValue(numer), FormatValue(denom))
		}
		if new(big.Int).GCD(nil, nil, new(big.Int).Abs(big.NewInt(numer.Const.Int)), big.NewInt(denom.Const.Int)).Cmp(big.NewInt(1)) != 0 {
			t.Fatalf("numer(%s)/denom(%s) = %s/%s is not in lowest terms", FormatValue(x), FormatValue(x), FormatValue(numer), FormatValue(denom))
		}
		back, err := applyLibrary(t, "RationalFunctions::rat", numer, denom)
		if err != nil {
			t.Fatalf("rat(%s, %s) = error %v", FormatValue(numer), FormatValue(denom), err)
		}
		if back.Const.Kind != semantics.ValReal || back.Const.Real != asReal(x.Const) {
			t.Fatalf("rat(numer(%s), denom(%s)) = %s, want %v", FormatValue(x), FormatValue(x), FormatValue(back), asReal(x.Const))
		}
		same, err := applyLibrary(t, "RationalFunctions::==", back, x)
		if err != nil || !valueIdentical(same, constBool(true)) {
			t.Fatalf("RationalFunctions::'=='(rat(numer(x), denom(x)), x) for x = %s is %s, %v; want true", FormatValue(x), FormatValue(same), err)
		}
	}
}

// ToString and ToReal round-trip: a Real reads back as the same Real.
func TestRealToStringRoundTrips(t *testing.T) {
	for _, x := range []float64{0, 1, -1.5, 0.1 + 0.2, 1e21, 1e-7, math.MaxFloat64, math.SmallestNonzeroFloat64, 123456789.125} {
		text, err := applyLibrary(t, "RealFunctions::ToString", constReal(x))
		if err != nil {
			t.Fatalf("ToString(%v) = error %v", x, err)
		}
		back, err := applyLibrary(t, "RealFunctions::ToReal", text)
		if err != nil {
			t.Fatalf("ToReal(%q) = error %v", text.Str(), err)
		}
		if back.Const.Kind != semantics.ValReal || back.Const.Real != x {
			t.Fatalf("ToReal(ToString(%v)) = %s, want %v", x, FormatValue(back), x)
		}
	}
}

// Every conversion failure is a typed error naming the function.
func TestConversionFunctionErrors(t *testing.T) {
	cases := []struct {
		fn   string
		args []Value
		want error
	}{
		{"BooleanFunctions::ToBoolean", []Value{NewStringValue("yes")}, ErrInvalidNotation},
		{"BooleanFunctions::ToBoolean", []Value{NewStringValue("True")}, ErrInvalidNotation},
		{"BooleanFunctions::ToBoolean", []Value{NewStringValue(" false ")}, ErrInvalidNotation},
		{"BooleanFunctions::ToBoolean", []Value{constInt(1)}, ErrTypeMismatch},
		{"IntegerFunctions::ToInteger", []Value{NewStringValue("7.0")}, ErrInvalidNotation},
		{"IntegerFunctions::ToInteger", []Value{NewStringValue("")}, ErrInvalidNotation},
		{"IntegerFunctions::ToInteger", []Value{NewStringValue("0x10")}, ErrInvalidNotation},
		{"IntegerFunctions::ToInteger", []Value{NewStringValue(" 7")}, ErrInvalidNotation},
		{"IntegerFunctions::ToInteger", []Value{NewStringValue("7\n")}, ErrInvalidNotation},
		{"NaturalFunctions::ToNatural", []Value{NewStringValue(" 0 ")}, ErrInvalidNotation},
		{"IntegerFunctions::ToInteger", []Value{NewStringValue("9223372036854775808")}, semantics.ErrArithmeticOverflow},
		{"IntegerFunctions::ToInteger", []Value{constInt(7)}, ErrTypeMismatch},
		{"NaturalFunctions::ToNatural", []Value{NewStringValue("-3")}, semantics.ErrArithmeticDomain},
		{"NaturalFunctions::ToNatural", []Value{NewStringValue("three")}, ErrInvalidNotation},
		{"IntegerFunctions::ToNatural", []Value{constInt(-1)}, semantics.ErrArithmeticDomain},
		{"IntegerFunctions::ToNatural", []Value{constReal(1)}, ErrTypeMismatch},
		{"RealFunctions::ToReal", []Value{NewStringValue("abc")}, ErrInvalidNotation},
		{"RealFunctions::ToReal", []Value{NewStringValue("1.5.2")}, ErrInvalidNotation},
		{"RealFunctions::ToReal", []Value{NewStringValue("1e")}, ErrInvalidNotation},
		{"RealFunctions::ToReal", []Value{NewStringValue("Inf")}, ErrInvalidNotation},
		{"RealFunctions::ToReal", []Value{NewStringValue("NaN")}, ErrInvalidNotation},
		{"RealFunctions::ToReal", []Value{NewStringValue("0x1p-2")}, ErrInvalidNotation},
		{"RealFunctions::ToReal", []Value{NewStringValue(" 1.5 ")}, ErrInvalidNotation},
		{"RealFunctions::ToReal", []Value{NewStringValue("1.5\t")}, ErrInvalidNotation},
		{"RationalFunctions::ToRational", []Value{NewStringValue(" 1.5 ")}, ErrInvalidNotation},
		{"RealFunctions::ToReal", []Value{NewStringValue("1e400")}, semantics.ErrArithmeticOverflow},
		{"RealFunctions::ToReal", []Value{NewStringValue("1e-400")}, semantics.ErrArithmeticOverflow},
		{"RealFunctions::ToReal", []Value{NewStringValue("-2e-324")}, semantics.ErrArithmeticOverflow},
		{"RealFunctions::ToReal", []Value{NewStringValue("0." + strings.Repeat("0", 330) + "1")}, semantics.ErrArithmeticOverflow},
		{"RationalFunctions::ToRational", []Value{NewStringValue("1e-400")}, semantics.ErrArithmeticOverflow},
		{"RealFunctions::ToReal", []Value{constReal(1)}, ErrTypeMismatch},
		{"RationalFunctions::ToRational", []Value{NewStringValue("1/3")}, ErrInvalidNotation},
		{"RealFunctions::ToInteger", []Value{constReal(1e19)}, semantics.ErrArithmeticOverflow},
		{"RealFunctions::ToInteger", []Value{NewStringValue("1")}, ErrTypeMismatch},
		{"RealFunctions::re", []Value{NewStringValue("1")}, ErrTypeMismatch},
		{"NaturalFunctions::ToString", []Value{constInt(-1)}, ErrTypeMismatch},
		{"IntegerFunctions::ToString", []Value{constReal(1)}, ErrTypeMismatch},
		{"BaseFunctions::ToString", []Value{constSequence(1, 2)}, ErrMultiplicityViolation},
		{"BaseFunctions::ToString", []Value{{Kind: ValInstance, Instance: 3}}, ErrTypeMismatch},
		{"BaseFunctions::ToString", []Value{cx(1, 1)}, ErrUnevaluableLibraryFunction},
		{"RationalFunctions::gcd", []Value{constReal(0.5), constInt(2)}, semantics.ErrArithmeticDomain},
		{"RationalFunctions::gcd", []Value{constInt(math.MinInt64), constInt(0)}, semantics.ErrArithmeticOverflow},
		{"RationalFunctions::gcd", []Value{constInt(math.MinInt64), constInt(math.MinInt64)}, semantics.ErrArithmeticOverflow},
		{"RationalFunctions::gcd", []Value{constReal(math.Ldexp(1, 63)), constReal(0)}, semantics.ErrArithmeticOverflow},
		{"RationalFunctions::gcd", []Value{constReal(1e19), constReal(1e19)}, semantics.ErrArithmeticOverflow},
		{"RationalFunctions::gcd", []Value{constReal(-1e300), constInt(math.MinInt64)}, semantics.ErrArithmeticOverflow},
		{"RationalFunctions::gcd", []Value{constReal(math.Inf(1)), constInt(2)}, semantics.ErrArithmeticDomain},
		{"RationalFunctions::rat", []Value{constInt(1), constInt(0)}, ErrDivisionByZero},
		{"RationalFunctions::rat", []Value{constInt(0), constInt(0)}, ErrDivisionByZero},
		{"RationalFunctions::rat", []Value{constReal(1), constInt(3)}, ErrTypeMismatch},
		{"RationalFunctions::rat", []Value{constInt(1), constReal(3)}, ErrTypeMismatch},
		{"RationalFunctions::rat", []Value{NewStringValue("1"), constInt(3)}, ErrTypeMismatch},
		{"RationalFunctions::numer", []Value{NewStringValue("0.5")}, ErrTypeMismatch},
		{"RationalFunctions::denom", []Value{constSequence(1, 2)}, ErrTypeMismatch},
		{"RationalFunctions::numer", []Value{constReal(1e19)}, semantics.ErrArithmeticOverflow},
		{"RationalFunctions::numer", []Value{constReal(math.Ldexp(1, 63))}, semantics.ErrArithmeticOverflow},
		{"RationalFunctions::denom", []Value{constReal(math.Ldexp(1, -63))}, semantics.ErrArithmeticOverflow},
		{"RationalFunctions::denom", []Value{constReal(0.0001)}, semantics.ErrArithmeticOverflow},
		{"RationalFunctions::denom", []Value{constReal(math.SmallestNonzeroFloat64)}, semantics.ErrArithmeticOverflow},
		{"RationalFunctions::numer", []Value{constReal(math.Inf(1))}, semantics.ErrArithmeticDomain},
		{"RationalFunctions::denom", []Value{constReal(math.NaN())}, semantics.ErrArithmeticDomain},
	}
	for _, tc := range cases {
		t.Run(tc.fn+"/"+FormatValue(tc.args[0]), func(t *testing.T) {
			_, err := applyLibrary(t, tc.fn, tc.args...)
			if !errors.Is(err, tc.want) {
				t.Fatalf("%s error = %v, want %v", tc.fn, err, tc.want)
			}
			if !strings.Contains(err.Error(), writtenName(tc.fn)) {
				t.Fatalf("%s error %q does not name the function", tc.fn, err)
			}
		})
	}
}
