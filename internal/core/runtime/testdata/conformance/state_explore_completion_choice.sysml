package test {
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): two unguarded
	// completion transitions out of `ready` are enabled by its one completion;
	// the library says one fires, not which. Both sides complete into `settled`,
	// so the final state is fixed and the state visited between, and `side`, are open.
	state Switch {
		attribute side : Integer = 0;

		entry; then ready;
		state ready;
		state left {
			entry { assign side := 1; }
		}
		state right {
			entry { assign side := 2; }
		}
		state settled;

		transition first ready then left;
		transition first ready then right;
		transition first left then settled;
		transition first right then settled;
	}
}
