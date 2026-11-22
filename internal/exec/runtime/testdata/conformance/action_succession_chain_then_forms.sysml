// The target of a positional `then`, a guarded `if … then` and an `else` is a connector end
// (SysML.xtext TargetSuccession, TransitionSuccession), so it may be a feature chain too.
package P {
	private import ScalarValues::*;

	part def Vehicle {
		attribute n : Integer = 0;
		action close { assign n := n + 1; }
		action lock { assign n := n * 10; }
		action open { assign n := n + 100; }
	}

	action def S {
		part v : Vehicle;
		in go : Boolean = true;
		out result : Integer;
		action read { assign result := v.n; }

		first start;
		then v.close;
		first v.close then pick;
		decide pick;
			if go then v.lock;
			else v.open;
		first v.lock then read;
		succession first v.open then read;
		first read then done;
	}
}
