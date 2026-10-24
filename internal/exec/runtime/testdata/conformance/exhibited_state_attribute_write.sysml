// A performed action of the exhibiting part assigns an attribute of the
// exhibited machine through the exhibit usage (`g.missDistance`): the exhibit
// usage names the one performance, so the write reaches the running machine,
// whose change trigger on that attribute fires, and the Abort sent next finds
// the machine in `aborting`.
package test {
	private import ScalarValues::*;

	item def Abort;

	state def Guard {
		attribute missDistance : Real = 20000.0;
		attribute aborted : Boolean = false;
		entry; then cruise;
		state cruise;
		state aborting { entry assign aborted := true; }
		state safe;
		transition first cruise accept when missDistance < 10000.0 then aborting;
		transition first aborting accept Abort then safe;
	}

	part def Flight {
		exhibit state g : Guard;
		perform action nav {
			first start;
			then action degrade assign g.missDistance := 5000.0;
			then action a send new Abort();
			then done;
		}
	}
}
