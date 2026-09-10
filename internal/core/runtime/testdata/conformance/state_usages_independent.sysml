// Two usages of one state definition keep separate attribute values and
// separate current-state configurations: entering `two` performs its own
// `entry; then counting;`, bumping its own `hits` and not `one`'s.
package test {
	private import ScalarValues::*;

	state def Counter {
		attribute hits : Integer = 0;

		entry; then counting;
		state counting {
			entry action bump {
				assign hits := hits + 1;
			}
		}
	}

	state def Twice {
		entry; then one;
		state one : Counter;
		state two : Counter;
		transition first one.counting then two;
		transition first two.counting then done;
	}
}
