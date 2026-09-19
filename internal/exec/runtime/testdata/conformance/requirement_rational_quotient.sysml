package test {
	// A whole-number quotient inside a requirement's constraint is a
	// Rational for either sign.
	requirement QuotientIsRational {
		require constraint { -7 / 2 == -3.5 }
	}
}
