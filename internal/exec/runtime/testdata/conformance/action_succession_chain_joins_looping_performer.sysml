// A chained succession end to a looping action (§7.17.4, §7.17.6) orders the stated performer
// `s ::> v.spin`: the loop runs to its `until` once, before `read`; no second performance.
package P {
	private import ScalarValues::*;

	part def Vehicle {
		attribute n : Integer = 0;
		attribute turns : Integer = 0;
		action spin {
			first start;
			then loop action body {
				action inc assign n := n + 1;
			} until n >= 3;
			then assign turns := turns + 1;
			then done;
		}
	}

	action def Stated {
		part v : Vehicle;
		out result : Integer;
		out turns : Integer;
		perform action s ::> v.spin;
		action read { assign result := v.n; assign turns := v.turns; }

		first start then v.spin;
		first v.spin then read;
		first read then done;
	}
}
