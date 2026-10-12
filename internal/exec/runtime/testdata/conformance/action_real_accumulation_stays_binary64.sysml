package test {
	private import ScalarValues::*;

	// A Real feature holds binary64 values: each step rounds the exact 0.01 into
	// the Real once, so a thousand steps stay a double rather than an exact
	// Rational whose denominator would grow, and reach 9.999999999999831, not 10.
	action accumulate {
		out attribute total : Real = 0.0;
		out attribute reachesTen : Boolean = false;

		first start;
		then action sum {
			for i in 1..1000 {
				assign total := total + 0.01;
			}
			assign reachesTen := total == 10;
		}
		then done;
	}
}
