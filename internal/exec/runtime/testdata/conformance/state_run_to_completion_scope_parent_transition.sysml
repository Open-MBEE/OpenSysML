// A transition declared by the machine body is outside a nested held scope.
package test {
	private import ScalarValues::*;

	item def Start;
	item def Go;

	state Machine {
		attribute hits : Integer = 0;
		entry; then start;
		state start;
		state work {
			attribute :>> runToCompletionScope default = self;
			state step {
				entry action count { assign hits := hits + 1; }
			}
			entry action { send new Go() to Machine; } then step;
		}
		state done;
		succession first start then work;
		transition first work accept Go then done;
	}
}
