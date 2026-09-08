package test {
	// Oracle (docs/project/behavior-semantic-oracle.md): the first transition out
	// of `idle` is enabled by Go; the second's guard has no result (it divides by
	// zero). A guard with no result is not true, so that transition is not
	// selected and the run is unchanged; the executor reports the guard it could
	// not evaluate.
	state Dispatcher {
		attribute level : Integer = 8;
		attribute route : Integer = 0;

		entry; then idle;
		state idle;
		state low {
			entry { assign route := 1; }
		}
		state high {
			entry { assign route := 2; }
		}

		transition first idle accept Go if level > 5 then low;
		transition first idle accept Go if 1 / (level - 8) > 0 then high;
	}
}
