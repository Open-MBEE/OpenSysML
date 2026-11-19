package test {
	private import ScalarValues::*;
	private import Stochastic::*;

	// A loop body stating its own flow carries Probability weights as an action
	// body does: every iteration draws the certain branch, never the zero-weight one.
	action route {
		attribute taken : Integer := 0;

		for i in 1..8 {
			first start;
			then decide select;
			first select then never { @Probability { p = 0.0; } }
			first select then always { @Probability { p = 1.0; } }

			action never { assign taken := taken + 100; }
			action always { assign taken := taken + 1; }
		}
	}
}
