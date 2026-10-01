package Test {
	private import ScalarValues::*;

	item def Start;
	item def Go;

	state Machine parallel {
		attribute hits : Integer = 0;

		state left {
			entry; then start;
			state start;
			state work {
				ref :>> runToCompletionScope = self;
				state step {
					entry action count { assign hits := hits + 1; }
				}
				entry action { send new Go() to Machine; } then step;
			}
			transition first start accept Start then work;
		}

		state right {
			entry; then idle;
			state idle;
			transition first idle accept Go then stop;
		}

		action stop terminate;
	}
}
