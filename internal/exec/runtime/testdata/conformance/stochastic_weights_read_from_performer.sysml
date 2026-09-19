// A Probability weight naming a feature is read from the performer when the decision
// is reached: parts redefining it to 1.0 and 0.0 take the branches those weigh, whatever the seed draws.
package test {
	private import ScalarValues::*;
	private import Stochastic::*;

	part def Analysis {
		attribute pFast : Real default = 0.5;
		attribute taken : Integer = 0;

		perform action route {
			first start;
			then decide select;
			first select then fast { @Probability { p = pFast; } }
			first select then slow { @Probability { p = 1.0 - pFast; } }

			action fast { assign taken := 1; }
			then done;

			action slow { assign taken := 2; }
			then done;
		}
	}

	part def Study {
		part fast : Analysis { attribute :>> pFast = 1.0; }
		part slow : Analysis { attribute :>> pFast = 0.0; }
	}
}
