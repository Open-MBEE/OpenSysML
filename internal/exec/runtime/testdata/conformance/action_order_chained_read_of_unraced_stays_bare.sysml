package test {
	private import ScalarValues::*;

	// Two branches race on `x`, and `sensor.raw`, written once before the fork,
	// is read after the join: the race is on another place, so what is read
	// through the part stays bare while `x` and the read of `x` stand open.
	part def Sensor {
		attribute raw : Integer = 0;
		attribute scaled : Integer = raw * 10;
	}

	action race {
		part sensor : Sensor;
		attribute x : Integer = 0;
		attribute z : Integer = 0;
		attribute zz : Integer = 0;
		attribute w : Integer = 0;

		first start;
		action prime { assign sensor.raw := 7; }
		fork split;
		action a { assign x := 1; }
		action b { assign x := 2; }
		join sync;
		action derive {
			assign z := sensor.raw;
			assign zz := sensor.scaled;
			assign w := x;
		}
		done;

		succession first start then prime;
		succession first prime then split;
		succession first split then a;
		succession first split then b;
		succession first a then sync;
		succession first b then sync;
		succession first sync then derive;
		succession first derive then done;
	}
}
