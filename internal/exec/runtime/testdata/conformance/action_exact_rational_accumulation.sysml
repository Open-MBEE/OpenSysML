package test {
	private import ScalarValues::*;

	// Rational assignments accumulate exactly, in order (KerML 1.0 §9.3.2.2.8):
	// ten steps of 0.1 reach exactly 1.0, and 1/1 + 1/2 + 1/3 + 1/4 is 25/12.
	action accumulate {
		out attribute tenths : Rational = 0.0;
		out attribute harmonic : Rational = 0.0;
		out attribute reachesOne : Boolean = false;

		first start;
		then action sum {
			for i in 1..10 {
				assign tenths := tenths + 0.1;
			}
			for n in 1..4 {
				assign harmonic := harmonic + 1 / n;
			}
			assign reachesOne := tenths == 1;
		}
		then done;
	}
}
