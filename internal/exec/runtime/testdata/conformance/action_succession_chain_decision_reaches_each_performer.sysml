// A decision's succession to a chained action two steps perform reaches both steps:
// the branch taken is one succession, so neither performer is an alternative of the other.
package P {
	private import ScalarValues::*;

	part def Driver {
		attribute m : Integer = 0;
		action ready { in d : Integer = 1; assign m := m + d; }
	}

	action def Two {
		part driver : Driver;
		out result : Integer;
		perform action d1 ::> driver.ready { in d = 10; }
		perform action d2 ::> driver.ready { in d = 100; }
		action read { assign result := driver.m; }
		action skip;

		first start;
		then decide;
			if true then driver.ready;
			else skip;
		first driver.ready then read;
		first read then done;
		first skip then done;
	}
}
