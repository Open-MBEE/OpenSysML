// A chained end spelt with short names is the chain the stated performer names in full,
// so the step with its bound input runs and no second performance is implied.
package P {
	private import ScalarValues::*;

	part def Vehicle {
		attribute m : Integer = 0;
		action <cl> close { in d : Integer = 1; assign m := m + d; }
	}

	action def Two {
		part <v> vehicle : Vehicle;
		out result : Integer;
		perform action c ::> vehicle.close { in d = 10; }
		action read { assign result := vehicle.m; }

		first start then v.cl;
		first vehicle.cl then read;
		first read then done;
	}
}
