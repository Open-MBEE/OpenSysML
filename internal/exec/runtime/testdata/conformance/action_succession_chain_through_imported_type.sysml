// A chained end reads its object's type as the body does, through the imports in scope:
// `v.close` is the action of a `Vehicle` the package imports.
package Lib {
	private import ScalarValues::*;

	part def Vehicle {
		attribute n : Integer = 0;
		action close { assign n := n + 1; }
	}
}

package P {
	private import ScalarValues::*;
	private import Lib::*;

	action def Run {
		part v : Vehicle;
		out result : Integer;
		action read { assign result := v.n; }

		first start then v.close;
		first v.close then read;
		first read then done;
	}
}
