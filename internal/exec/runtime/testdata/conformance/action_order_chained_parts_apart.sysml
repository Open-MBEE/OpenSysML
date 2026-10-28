// Two parts of one definition: the race on left.raw leaves right.raw, what is
// computed from it, and the write read from it where they stand.
package test {
	private import ScalarValues::*;
	part def Sensor { attribute raw : Integer = 0; attribute scaled : Integer = raw * 10; }
	action race {
		part left : Sensor;
		part right : Sensor;
		attribute z : Integer = 0;
		first start;
		fork split;
		action a { assign left.raw := 1; }
		action b { assign left.raw := 2; }
		join sync;
		action derive { assign z := right.raw; }
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
