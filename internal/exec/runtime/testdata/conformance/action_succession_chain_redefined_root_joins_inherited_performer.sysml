// A part redefined in each specialization is one feature, so a chained end through it
// is the performer the farthest general states, and what follows it reads its result.
package P {
	private import ScalarValues::*;

	part def Vehicle {
		action close { in d : Integer = 1; out r : Integer = d; }
	}

	action def Base {
		part v : Vehicle;
		perform action c ::> v.close { in d = 10; }
	}

	action def Mid :> Base {
		part :>> v;
	}

	action def Run :> Mid {
		part :>> v;
		out result : Integer;
		action read { assign result := c.r; }

		first start then v.close;
		first v.close then read;
		first read then done;
	}
}
