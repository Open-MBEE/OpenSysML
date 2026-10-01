// Two transitions out of idle wait on the same signal, weighted 0.3 and 0.7:
// a seeded run draws the branch by weight from the modeled stream.
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
