// The branches out of a choice pseudostate are weighted too: once the incoming
// transition fires, the choice is drawn by the weights its branches state.
package test {
	private import ScalarValues::*;
	private import Stochastic::*;
	state def Machine {
		attribute log : Integer = 0;
		entry; then idle;
		state idle;
		choice pick;
		state left {
			entry { assign log := 10 * log + 3; }
		}
		state right {
			entry { assign log := 10 * log + 4; }
		}
		transition first idle accept go then pick;
		transition first pick then left { @Probability { p = 0.6; } }
		transition first pick then right { @Probability { p = 0.4; } }
	}
}
