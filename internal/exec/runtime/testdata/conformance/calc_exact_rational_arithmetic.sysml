// A Rational is exact (KerML 1.0 §9.3.2.2.8; RationalFunctions): a decimal
// literal is the Rational it spells, Integer `/` and the Rational operators
// compute without rounding, a whole Rational stays a Rational, and only a
// Real-typed function (sqrt) yields a binary64 Real.
package test {
	private import ScalarValues::*;

	calc def ExactRationals {
		out tenthsSum = 0.1 + 0.2;
		out tenthsSumIsThreeTenths = 0.1 + 0.2 == 0.3;
		out third = 1 / 3;
		out thirdTimesThree = 1 / 3 * 3;
		out thirdTimesThreeIsOne = 1 / 3 * 3 == 1;
		out thirds = 1 / 3 + 1 / 3;
		out difference = 0.3 - 0.1;
		out quotient = 0.1 / 0.3;
		out power = (2 / 3) ** 3;
		out inversePower = (2 / 3) ** -2;
		out exponentLiteral = 1.5e3;
		out smallExponentLiteral = 25e-3;
		out thirdIsBelow = 1 / 3 < 0.3334;
		out absolute = RationalFunctions::abs(-1 / 3);
		out floored = RationalFunctions::floor(-7 / 2);
		out rounded = RationalFunctions::round(5 / 2);
		out largest = RationalFunctions::max(1 / 3, 0.3);
		out smallest = RationalFunctions::min(1 / 3, 0.3);
		out total = RationalFunctions::sum((0.1, 0.2, 0.3));
		out product = RationalFunctions::product((1 / 2, 2 / 3, 3 / 4));
		out gcd = RationalFunctions::gcd(6, 4);
		out text = RationalFunctions::ToString(1 / 3);
		out fromText = RationalFunctions::ToRational("2/6");
		out fromDecimalText = RationalFunctions::ToRational("0.125");
		out root = RealFunctions::sqrt(0.25);
	}

	calc c : ExactRationals;
}
