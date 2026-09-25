// A `terminate` in a braced `entry { … }` ends that block alone: the named entry
// actions declared before and after it run to their ends, so `head` and `tail`
// both write and only the braced block's trailing assignment is cut.
package Test {
	private import ScalarValues::*;

	state def Machine {
		attribute a : Integer = 0;
		attribute log : String = "";

		entry; then s;

		state s {
			entry action head { assign log := log + "head;"; }
			entry { assign a := 1; terminate; assign a := 9; }
			entry action tail { assign log := log + "tail;"; }
		}
	}
}
