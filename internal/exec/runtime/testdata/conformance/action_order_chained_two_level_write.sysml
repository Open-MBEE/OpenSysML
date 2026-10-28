package test {
	private import ScalarValues::*;

	// Two branches each write `car.engine.rpm`, two parts deep: the race is on
	// the place the whole chain reaches, and the value read from it after the
	// join stands on it, while `car.mass`, which neither branch touches, stays bare.
	part def Engine { attribute rpm : Integer = 0; }
	part def Car {
		attribute mass : Integer = 1200;
		part engine : Engine;
	}

	action race {
		part car : Car;
		attribute z : Integer = 0;
		attribute m : Integer = 0;

		first start;
		fork split;
		action a { assign car.engine.rpm := 1; }
		action b { assign car.engine.rpm := 2; }
		join sync;
		action derive {
			assign z := car.engine.rpm;
			assign m := car.mass;
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
