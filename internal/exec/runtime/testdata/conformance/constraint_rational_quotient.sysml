package test {
	// A whole-number quotient inside a constraint is a Rational, so the
	// exact comparison against 3.5 holds.
	constraint quotientIsRational {
		7 / 2 == 3.5 and 9 / 3 == 3.0
	}
}
