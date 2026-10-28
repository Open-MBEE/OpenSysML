package test {
	private import ScalarValues::*;

	// Two branches between one fork and one join each write `sensor.raw`, a
	// feature reached through the action's part: the library orders neither
	// write before the other, so the chained place stands open, as does
	// `sensor.scaled`, valued by an expression over it, and the two values the
	// read after the join writes from them.
	part def Sensor {
		attribute raw : Integer = 0;
		attribute scaled : Integer = raw * 10;
	}

	action race {
		part sensor : Sensor;
		attribute z : Integer = 0;
		attribute zz : Integer = 0;

		first start;
		fork split;
		action a { assign sensor.raw := 1; }
		action b { assign sensor.raw := 2; }
		join sync;
		action derive {
			assign z := sensor.raw;
			assign zz := sensor.scaled;
		}
		done;

		succession first start then split;
		succession first split then a;
		succession first split then b;
		succession first a then sync;
		succession first b then sync;
		succession first sync then derive;
		succession first derive then done;
	}
}
