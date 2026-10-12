package test {
	private import ScalarValues::*;

	calc def Twice {
		in x : Rational;
		return : Rational = x * 2;
	}
	calc twice : Twice;
}
