// A specialization inherits a succession whose end chains through two parts; the
// end starts no flow of its own, so the action it reaches begins with the body.
package P {
	private import ScalarValues::*;

	part def Door {
		attribute n : Integer = 0;
		action close { assign n := n + 1; }
	}
	part def Car { part door : Door; }

	action def Base {
		part car : Car;
		out result : Integer;
		action read { assign result := car.door.n; }

		first car.door.close then read;
		first read then done;
	}
	action def Derived :> Base;
}
