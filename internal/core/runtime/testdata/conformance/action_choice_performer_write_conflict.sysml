package test {
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): both branches of the
	// performed action write the performing part's `mark` in the same step. The
	// value is the same either way, but the library leaves which write stands
	// open, so the executor reports the conflict on the performer all the same.
	part def Host {
		attribute mark : Integer = 0;

		perform action marking {
			first start;
			fork split;
			action left { assign mark := 1; }
			action right { assign mark := 1; }
			join sync;
			done;

			succession first start then split;
			succession first split then left;
			succession first split then right;
			succession first left then sync;
			succession first right then sync;
			succession first sync then done;
		}
	}
}
