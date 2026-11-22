// A step stated to perform the chained action (`dr ::> driver.ready`) is an occurrence of it, so
// an end naming `driver.ready` orders that step, no second performance: `m` is written once.
package P {
	private import ScalarValues::*;

	part def Driver {
		attribute m : Integer = 0;
		action ready { in d : Integer = 1; assign m := m + d; }
	}

	action def One {
		part driver : Driver;
		out result : Integer;
		perform action dr ::> driver.ready { in d = 10; }
		action read { assign result := driver.m; }

		first start then driver.ready;
		first driver.ready then read;
		first read then done;
	}
}
