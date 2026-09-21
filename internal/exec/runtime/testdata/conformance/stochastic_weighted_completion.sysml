// The two completion transitions out of decide are weighted: entering it runs
// its entry, queues the one completion event, and the dispatch of that event
// draws the branch by weight once, under run-to-completion.
package test {
	private import ScalarValues::*;
	private import Stochastic::*;
	state def Machine {
		attribute log : Integer = 0;
		entry; then decide;
		state decide {
			entry { assign log := 10 * log + 1; }
		}
		state left {
			entry { assign log := 10 * log + 3; }
		}
		state right {
			entry { assign log := 10 * log + 4; }
		}
		transition first decide then left { @Probability { p = 0.4; } }
		transition first decide then right { @Probability { p = 0.6; } }
	}
}
