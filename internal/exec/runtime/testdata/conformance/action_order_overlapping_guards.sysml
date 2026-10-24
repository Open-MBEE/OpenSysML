package test {
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): both guards out of the
	// decision hold at level = 0, and the library picks neither succession over
	// the other, so either branch may run; each writes `mode` its own value, so
	// `mode` is order-dependent and a default run reports it only up to that choice.
	action pick {
		attribute level : Integer = 0;
		attribute mode : Integer = 0;

		first start;

		action high {
			assign mode := 1;
		}

		action low {
			assign mode := 2;
		}

		done;

		succession first start then choose;
		succession first high then done;
		succession first low then done;

		decide choose;
		if level >= 0 then high;
		if level <= 0 then low;
	}
}
