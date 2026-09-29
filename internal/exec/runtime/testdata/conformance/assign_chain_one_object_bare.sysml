// A bare `in ref` parameter stores its object as a collection of one, and a
// chained write through it reaches that one object, as a read already does:
// mark writes the performer's seen.
package test {
	private import ScalarValues::*;

	part def Vehicle { attribute seen : Integer default 0; }
	part def Car :> Vehicle {
		action def Mark { in ref vehicle : Vehicle; first write; action write { assign vehicle.seen := 1; } }
		exhibit state run {
			entry; then marking;
			state marking { entry action mark : Mark { in ref :>> vehicle = this; } }
		}
	}
	part car : Car;
}
