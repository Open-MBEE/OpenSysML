package test {
	private import ScalarValues::*;

	item def Ping;

	state Machine {
		attribute hits : Integer = 0;
		entry; then start;
		state start;

		state working {
			attribute :>> isRunToCompletion = false;
			state step {
				entry action count { assign hits := hits + 1; }
			}
			entry action { send new Ping() to Machine; } then step;
		}
		state done;
		transition first working accept Ping then done;
		succession first start then working;
	}
}
