// A redefinition restating the library default masks the redefinition it
// inherits, on the machine and on a substate: the inherited `false` is not what
// the machine runs under, so the machine is lowered and runs unchanged.
package test {
	private import ScalarValues::*;

	attribute def Go;

	state def Relaxed {
		attribute :>> isRunToCompletion = false;
	}

	state def Base {
		attribute :>> isRunToCompletion = false;
		attribute hits : Integer = 0;

		entry; then idle;
		state idle : Relaxed {
			attribute :>> isRunToCompletion = true;
			entry action bump {
				assign hits := hits + 1;
			}
		}
		transition first idle accept Go then done;
		state done;
	}

	state def Machine :> Base {
		attribute :>> isRunToCompletion = true;
	}
}
