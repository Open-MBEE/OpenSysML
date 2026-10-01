// A machine-body transition may name a nested source without becoming part of
// the nested state's run-to-completion scope.
package test {
	private import ScalarValues::*;

	item def Go;

	state Machine {
		attribute hits : Integer = 0;
		entry; then start;
		state start;
		state work {
			attribute :>> runToCompletionScope default = self;
			entry; then inner;
			state inner {
				state leaf {
					entry action count { assign hits := hits + 1; }
				}
				entry action { send new Go() to Machine; } then leaf;
			}
		}
		state done;
		succession first start then work;
		transition first work.inner accept Go then done;
	}
}
