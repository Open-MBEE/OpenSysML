// An accept in a loop body of an action a part performs parks that performance as
// the part's own (Performances.kerml: a performance an occurrence performs is
// enclosed by it), so a sibling object's sends addressed to the part wake it, one
// per iteration, until the loop's own condition ends it.
package test {
	private import ScalarValues::*;

	item def Tick {
		attribute k : Integer;
	}

	part def Listener {
		attribute total : Integer = 0;
		attribute heard : Integer = 0;
		perform action listen {
			while heard < 2 {
				accept t : Tick;
				assign total := total + t.k;
				assign heard := heard + 1;
			}
		}
	}

	action controller {
		attribute got : Integer = 0;
		part listener : Listener;
		part emitter {
			perform action emit {
				first start;
				action one send new Tick(k = 3) to listener;
				action two send new Tick(k = 4) to listener;
				done;
				succession first start then one;
				succession first one then two;
				succession first two then done;
			}
		}
		first start;
		action poke send new Tick(k = 0) to emitter;
		action settle;
		action collect { assign got := listener.total; }
		done;
		succession first start then poke;
		succession first poke then settle;
		succession first settle then collect;
		succession first collect then done;
	}
}
