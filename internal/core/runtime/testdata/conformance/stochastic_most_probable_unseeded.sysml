package test {
	private import ScalarValues::*;
	private import Stochastic::*;

	// The same weighted decision under a policy that draws nothing: the run takes
	// the most probable branch, so an unseeded run stays deterministic.
	action route {
		attribute taken : Integer = 0;

		first start;
		then decide select;
		first select then slow { @Probability { p = 0.3; } }
		first select then fast { @Probability { p = 0.7; } }

		action fast { assign taken := 1; }
		then done;

		action slow { assign taken := 2; }
		then done;
	}
}
