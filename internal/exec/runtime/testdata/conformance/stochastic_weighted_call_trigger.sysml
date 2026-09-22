// Two transitions out of idle wait on the same call, weighted by the priority
// the invocation carries: the draw reads the trigger's bound argument.
package test {
	private import ScalarValues::*;
	private import Stochastic::*;
	state def Machine {
		attribute log : Real = 0.0;
		entry; then idle;
		state idle;
		state slow {
			entry { assign log := 10.0 * log + 1.0; }
		}
		state fast {
			entry { assign log := 10.0 * log + 2.0; }
		}
		transition first idle accept route(priority) then slow { @Probability { p = priority; } }
		transition first idle accept route(priority) then fast { @Probability { p = 1.0 - priority; } }
	}
}
