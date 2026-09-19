// A Rational is a binary64 here (docs/project/exact-rational-evaluation.md):
// rat(n, d) is the quotient `/` computes, and numer/denom read the exact
// numerator and denominator of that binary64, so rat(numer(x), denom(x)) is x
// while numer(0.1) is the 16-digit numerator of the double nearest 0.1.
package test {
	private import ScalarValues::*;

	calc def RationalTerms {
		out third = RationalFunctions::rat(1, 3);
		out thirdIsTheQuotient = RationalFunctions::rat(1, 3) == 1 / 3;
		out sixQuarters = RationalFunctions::rat(6, 4);
		out negative = RationalFunctions::rat(-3, 4);
		out negativeDenominator = RationalFunctions::rat(3, -4);
		out zero = RationalFunctions::rat(0, 1);
		out whole = RationalFunctions::rat(4, 2);
		out tenth = RationalFunctions::rat(3602879701896397, 36028797018963968) == 0.1;
		out numerOfThreeQuarters = RationalFunctions::numer(0.75);
		out denomOfThreeQuarters = RationalFunctions::denom(0.75);
		out numerOfNegative = RationalFunctions::numer(-0.75);
		out denomOfNegative = RationalFunctions::denom(-0.75);
		out numerOfSixQuarters = RationalFunctions::numer(RationalFunctions::rat(6, 4));
		out denomOfSixQuarters = RationalFunctions::denom(RationalFunctions::rat(6, 4));
		out numerOfZero = RationalFunctions::numer(0.0);
		out denomOfZero = RationalFunctions::denom(0.0);
		out numerOfInteger = RationalFunctions::numer(2);
		out denomOfInteger = RationalFunctions::denom(2);
		out numerOfWholeRational = RationalFunctions::numer(2.0);
		out denomOfWholeRational = RationalFunctions::denom(2.0);
		out numerOfTenth = RationalFunctions::numer(0.1);
		out denomOfTenth = RationalFunctions::denom(0.1);
		out numerOfThird = RationalFunctions::numer(RationalFunctions::rat(1, 3));
		out denomOfThird = RationalFunctions::denom(1.0 / 3.0);
		out tenthRoundTrips = RationalFunctions::rat(RationalFunctions::numer(0.1), RationalFunctions::denom(0.1)) == 0.1;
		out thirdRoundTrips = RationalFunctions::rat(RationalFunctions::numer(1.0 / 3.0), RationalFunctions::denom(1.0 / 3.0)) == 1.0 / 3.0;
		out negativeRoundTrips = RationalFunctions::rat(RationalFunctions::numer(-0.1), RationalFunctions::denom(-0.1)) == -0.1;
		out integerRoundTrips = RationalFunctions::rat(RationalFunctions::numer(-7), RationalFunctions::denom(-7)) == -7;
		out asReal = RealFunctions::ToReal(RationalFunctions::ToString(RationalFunctions::rat(6, 4)));
		out asInteger = RationalFunctions::ToInteger(RationalFunctions::rat(7, 2));
	}

	calc c : RationalTerms;
}
