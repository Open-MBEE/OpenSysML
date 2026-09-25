// A transition's effect writes a feature an action reads: whether the action
// reads before or after the dispatch is dependent, and both readings are reached.
package test {
	private import ScalarValues::*;

	part def Lamp {
		attribute lit : Boolean = false;
	}

	part lamp : Lamp;

	action watcher {
		attribute saw : Boolean = false;
		attribute other : Integer = 0;
		first start;
		fork split;
		action look { assign saw := lamp.lit; }
		action busy { assign other := 1; }
		join sync;
		done;
		succession first start then split;
		succession first split then look;
		succession first split then busy;
		succession first look then sync;
		succession first busy then sync;
		succession first sync then done;
	}

	state lighter {
		entry; then dark;
		state dark;
		state shining;
		transition first dark do assign lamp.lit := true then shining;
	}
}
