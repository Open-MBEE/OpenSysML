// A scoped self boundary holds the queued dispatch while the composite entry
// cascade is incomplete, so the nested transition runs after step is entered.
package test {
	private import ScalarValues::*;

	item def Ping;

	state Machine {
		attribute hits : Integer = 0;
		entry; then start;
		state start;
		state work {
			attribute :>> runToCompletionScope default = self;
			state step {
				entry action count { assign hits := hits + 1; }
			}
			entry action { send new Ping() to Machine; } then step;
			state done;
			transition first step accept Ping then done;
		}
		succession first start then work;
	}
}
