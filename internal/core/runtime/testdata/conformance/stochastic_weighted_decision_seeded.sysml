package test {
	private import ScalarValues::*;
	private import Stochastic::*;

	// A decision whose branches carry Probability weights: a seeded run draws the
	// branch by weight from the modeled stream, so a fixed model seed fixes the pick.
	action route {
		attribute taken : Integer = 0;

		first start;
		then decide select;
		first select then fast { @Probability { p = 0.7; } }
		first select then slow { @Probability { p = 0.3; } }

		action fast { assign taken := 1; }
		then done;

		action slow { assign taken := 2; }
		then done;
	}
}
