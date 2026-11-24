// A general's private `v` is not the specializing action's `v`: a chain through each
// is its own action, so each is performed once on its own object.
package P {
	private import ScalarValues::*;

	part def Vehicle {
		attribute n : Integer = 0;
		action close { assign n := n + 1; }
	}

	action def Base {
		private part v : Vehicle;
		attribute b : Integer = 0;
		action a;
		action readB { assign b := v.n; }

		first start then v.close;
		first v.close then readB;
		first readB then a;
	}

	action def Run :> Base {
		private part v : Vehicle;
		out result : Integer;
		action read { assign result := v.n + 10 * b; }

		first a then v.close;
		first v.close then read;
		first read then done;
	}
}
