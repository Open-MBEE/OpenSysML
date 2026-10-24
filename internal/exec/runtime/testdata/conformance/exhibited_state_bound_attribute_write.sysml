// The exhibited machine's attribute is bound to the exhibiting part's
// (`:>> missDistance = Flight::missDistance`); a performed action's write
// through the exhibit usage still reaches the running machine, ending the
// binding as an assignment to the machine's own attribute would.
package test {
	private import ScalarValues::*;

	item def Abort;

	state def Guard {
		attribute missDistance : Real = 0.0;
		attribute aborted : Boolean = false;
		entry; then cruise;
		state cruise;
		state aborting { entry assign aborted := true; }
		state safe;
		transition first cruise accept when missDistance < 10000.0 then aborting;
		transition first aborting accept Abort then safe;
	}

	part def Flight {
		attribute missDistance : Real = 20000.0;
		exhibit state g : Guard {
			:>> missDistance = Flight::missDistance;
		}
		perform action nav {
			first start;
			then action degrade assign g.missDistance := 5000.0;
			then action a send new Abort();
			then done;
		}
	}
}
