package test {
	// `loop … until` tests its condition after each iteration, so the body runs
	// at least once. Halving is Rational: a whole-number quotient is one too.
	calc halve {
		in n: Integer;
		attribute i : Rational = n;
		loop {
			assign i := i / 2;
		} until i <= 1;
		return : Rational = i;
	}
}
