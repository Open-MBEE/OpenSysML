package test {
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): `gate` is reached by one fork
	// branch directly, by the other through `prep`, and again from `more` on every pass
	// the guard keeps; every arrival traverses it. `more` counts each pass before its
	// guards read the count, so two passes select `gate` and two select `done` however
	// the two tokens interleave: merged = 4, worked = 4, passes = 4.
	action forkIntoLoop {
		attribute merged : Integer = 0;
		attribute worked : Integer = 0;
		attribute passes : Integer = 0;

		first start;
		fork split;
		action prep;
		merge gate { assign merged := merged + 1; }
		action work { assign worked := worked + 1; }
		decide more { assign passes := passes + 1; }
		done;

		succession first start then split;
		succession first split then gate;
		succession first split then prep;
		succession first prep then gate;
		succession first gate then work;
		succession first work then more;
		succession first more if passes < 3 then gate;
		succession first more if passes >= 3 then done;
	}
}
