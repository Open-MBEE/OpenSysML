// Two steps performing the chained action are two occurrences of it: a succession end naming
// `driver.ready` relates each, so both follow `start` and `read` waits for both.
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

		first start then driver.ready;
		first driver.ready then read;
		first read then done;
	}
}
