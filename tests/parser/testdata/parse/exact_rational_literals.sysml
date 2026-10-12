package P {
	private import ScalarValues::*;

	calc def ExactRationals {
		out tenthsSum : Rational = 0.1 + 0.2;
		out third : Rational = 1 / 3;
		out exponent : Rational = 1.5e3;
		out small : Rational = 25E-3;
		out leadingPoint : Rational = .5;
		out power : Rational = (2 / 3) ** -2;
		out terms : Integer = RationalFunctions::numer(RationalFunctions::rat(6, 4));
		out root : Real = RealFunctions::sqrt(0.25);
		out unbounded = *;
	}
}
