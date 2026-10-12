package test {
	private import ScalarValues::*;

	calc def Third {
		in x : Rational;
		return : Rational = x / 3;
	}
	calc third : Third;
}
