// One feature under two names: a write through the declared name meets a read
// through the short name, and a write through a redefining feature meets a read
// through the name it redefines. Each branch takes a further step before the
// join so the aliased moves are not already dependent through it.
package test {
	private import ScalarValues::*;

	action def Base {
		attribute base : Integer = 0;
	}

	action aliased : Base {
		attribute <sx> x : Integer = 0;
		attribute y :>> base;
		attribute seenX : Integer = -1;
		attribute seenBase : Integer = -1;
		first start;
		fork split;
		action wx { assign x := 1; }
		action rx { assign seenX := sx; }
		action wy { assign y := 1; }
		action rb { assign seenBase := base; }
		action ax;
		action arx;
		action ay;
		action ab;
		join sync;
		done;
		succession first start then split;
		succession first split then wx;
		succession first split then rx;
		succession first split then wy;
		succession first split then rb;
		succession first wx then ax;
		succession first rx then arx;
		succession first wy then ay;
		succession first rb then ab;
		succession first ax then sync;
		succession first arx then sync;
		succession first ay then sync;
		succession first ab then sync;
		succession first sync then done;
	}
}
