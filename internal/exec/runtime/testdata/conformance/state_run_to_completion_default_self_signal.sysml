// The library defaults keep the self-signal dispatch after the entry cascade,
// providing the default twin for the explicitly non-run-to-completion case.
package test {
	private import ScalarValues::*;

	item def Ping;

	state Machine {
		attribute hits : Integer = 0;
		entry; then start;
		state start;

		state working {
			state step {
				entry action count { assign hits := hits + 1; }
			}
			entry { send Ping to Machine; }
			entry; then step;
		}
		state done;
		transition first working accept Ping then done;
		succession first start then working;
	}
}
