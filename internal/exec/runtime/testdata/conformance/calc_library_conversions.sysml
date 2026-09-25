// The Kernel Function Library conversions between String, Boolean and the
// numeric scalars, called by qualified name, each in the kind it declares.
package test {
	private import ScalarValues::*;

	enum def Color { red; green; }

	calc def Conversions {
		out realFromString = RealFunctions::ToReal("1.5");
		out realFromExponent = RealFunctions::ToReal("-2.5e3");
		out rationalFromString = RationalFunctions::ToRational("0.25");
		out integerFromString = IntegerFunctions::ToInteger("-7");
		out naturalFromString = NaturalFunctions::ToNatural("42");
		out booleanFromString = BooleanFunctions::ToBoolean("true");
		out stringFromInteger = IntegerFunctions::ToString(42);
		out stringFromNatural = NaturalFunctions::ToString(7);
		out stringFromBoolean = BooleanFunctions::ToString(false);
		out stringFromReal = RealFunctions::ToString(2.5);
		out stringFromWholeReal = RealFunctions::ToString(1.0);
		out stringFromRational = RationalFunctions::ToString(0.5);
		out stringFromAnything = BaseFunctions::ToString(3);
		out stringFromNull = BaseFunctions::ToString(null);
		out stringFromLiteral = BaseFunctions::ToString(Color::red);
		out roundTrip = RealFunctions::ToReal(RealFunctions::ToString(0.1)) == 0.1;
		out naturalFromInteger = IntegerFunctions::ToNatural(9);
		out integerFromReal = RealFunctions::ToInteger(2.7);
		out integerFromNegativeReal = RealFunctions::ToInteger(-2.7);
		out integerFromRational = RationalFunctions::ToInteger(0.75);
		out rationalFromReal = RealFunctions::ToRational(0.75);
		out rationalFromInteger = RealFunctions::ToRational(2);
		out rationalFromIntegerIsReal = RealFunctions::ToRational(2) === 2.0;
		out realPart = RealFunctions::re(2.5);
		out imaginaryPart = RealFunctions::im(2.5);
		out argument = RealFunctions::arg(2.5);
		out floored = RationalFunctions::floor(-0.5);
		out rounded = RationalFunctions::round(2.5);
		out divisor = RationalFunctions::gcd(-12, 18);
		out divisorOfZeros = RationalFunctions::gcd(0, 0);
		out divisorOfLargeRationals = RationalFunctions::gcd(1.0e20, 6.0e18);
	}

	calc c : Conversions;
}
