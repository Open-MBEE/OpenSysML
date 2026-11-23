// A join node's one succession to a chained action two steps perform reaches both steps.
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
		fork f;
		join m;
		action p;
		action q;
		first start then f;
		first f then p;
		first f then q;
		first p then m;
		first q then m;
		first m then driver.ready;
		first driver.ready then read;
		first read then done;
	}
}
