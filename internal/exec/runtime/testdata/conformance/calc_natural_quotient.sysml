package test {
	// Natural operands divide as whole numbers do: the quotient is a
	// Rational, not a truncated Natural.
	calc def natQuotient {
		in a : ScalarValues::Natural;
		in b : ScalarValues::Natural;
		return : ScalarValues::Rational = a / b;
	}
}
