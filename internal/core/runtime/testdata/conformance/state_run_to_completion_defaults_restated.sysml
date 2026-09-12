// Redefinitions that restate the Kernel Semantic Library defaults —
// isRunToCompletion = true, runToCompletionScope = self (the whole machine) —
// say what the runtime does: the machine is lowered and runs unchanged.
package test {
	private import ScalarValues::*;

	attribute def Go;

	state def Machine {
		attribute :>> isRunToCompletion = true;
		ref :>> runToCompletionScope = self;
		attribute hits : Integer = 0;

		entry; then idle;
		state idle {
			attribute :>> isRunToCompletion = true;
			entry action bump {
				assign hits := hits + 1;
			}
		}
		transition first idle accept Go then done;
		state done;
	}
}
