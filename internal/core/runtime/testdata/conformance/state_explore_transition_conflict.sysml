package test {
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): two unguarded
	// transitions out of `idle` are enabled by one Go; the library says one
	// fires, not which. A second Go settles either side, so the final state is
	// fixed and the state visited between, and `side`, are open.
	state Switch {
		attribute side : Integer = 0;

		entry; then idle;
		state idle;
		state left {
			entry { assign side := 1; }
		}
		state right {
			entry { assign side := 2; }
		}
		state settled;

		transition first idle accept Go then left;
		transition first idle accept Go then right;
		transition first left accept Go then settled;
		transition first right accept Go then settled;
	}
}
