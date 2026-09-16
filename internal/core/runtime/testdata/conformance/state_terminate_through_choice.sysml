// A choice whose enabled branch leads into a terminate action ends the machine
// once the branch is taken: the segments' effects run in order, the source state
// is not exited, and the other branch's target is never entered.
package Test {
	private import ScalarValues::*;

	attribute def Check;

	state def Machine {
		attribute x : Integer = 0;
		attribute log : String = "";

		entry; then idle;
		state idle {
			exit { assign log := log + "idle(exit);"; }
		}
		state fine {
			entry { assign log := log + "fine(entry);"; }
		}
		choice verdict;
		action stop terminate;

		transition first idle accept Check do { assign x := 2; assign log := log + "check;"; } then verdict;
		transition first verdict if x > 1 do { assign log := log + "bad;"; } then stop;
		transition first verdict do { assign log := log + "good;"; } then fine;
	}
}
