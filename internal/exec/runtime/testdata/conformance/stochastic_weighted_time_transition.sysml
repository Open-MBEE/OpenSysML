// Two transitions out of idle wait on the same interval, weighted 0.9 and 0.1:
// one timer fires and the branch is drawn by weight.
package test {
	private import ScalarValues::*;
	private import SI::*;
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
		transition first idle accept after 5 [s] then slow { @Probability { p = 0.9; } }
		transition first idle accept after 5 [s] then fast { @Probability { p = 0.1; } }
	}
}
