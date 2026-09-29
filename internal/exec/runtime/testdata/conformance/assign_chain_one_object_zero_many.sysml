// An `in ref` parameter storing its object as a collection of one reads the
// same as its bare twin: a chained write through it reaches that one object.
package test {
	private import ScalarValues::*;

	part def Vehicle { attribute seen : Integer default 0; }
	part def Car :> Vehicle {
		action def Mark { in ref vehicle : Vehicle[0..*]; first write; action write { assign vehicle.seen := 1; } }
		exhibit state run {
			entry; then marking;
			state marking { entry action mark : Mark { in ref :>> vehicle = this; } }
		}
	}
	part car : Car;
}
