// The same weighted pair under a policy that draws nothing: the run takes the
// most probable transition, so an unseeded run stays deterministic.
package test {
	private import ScalarValues::*;
	private import Stochastic::*;
	state def Machine {
		attribute log : Integer = 0;
		entry; then idle;
		state idle;
		state slow {
			entry { assign log := 10 * log + 1; }
		}
		state fast {
			entry { assign log := 10 * log + 2; }
		}
		transition first idle accept go then slow { @Probability { p = 0.3; } }
		transition first idle accept go then fast { @Probability { p = 0.7; } }
	}
}
