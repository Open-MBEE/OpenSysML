// A machine-level false redefinition applies at initialization, where the
// queued Go signal can dispatch before the working entry cascade completes.
package test {
	private import ScalarValues::*;

	item def Go;

	state Machine {
		attribute :>> isRunToCompletion = false;
		attribute hits : Integer = 0;
		entry { send Go to Machine; }
		entry; then working;
		state working {
			state step {
				entry action count { assign hits := hits + 1; }
			}
			entry; then step;
		}
		state done;
		transition first working accept Go then done;
	}
}
