package test {
	private import ScalarValues::*;

	part def Sensor {
		attribute reading : Real = 0.0;
	}

	part s : Sensor;

	// Oracle (docs/project/behavior-semantic-oracle.md): both branches write the
	// sensor's `reading` through the chain `s.reading` in the same step; the
	// library leaves which write stands open, so the executor reports the
	// conflict on the object written and the value that stood.
	action calibrate {
		attribute seen : Real = 0.0;

		first start;
		fork split;
		action coarse { assign s.reading := 1.5; }
		action fine { assign s.reading := 2.5; }
		join settle;
		action read { assign seen := s.reading; }
		done;

		succession first start then split;
		succession first split then coarse;
		succession first split then fine;
		succession first coarse then settle;
		succession first fine then settle;
		succession first settle then read;
		succession first read then done;
	}
}
