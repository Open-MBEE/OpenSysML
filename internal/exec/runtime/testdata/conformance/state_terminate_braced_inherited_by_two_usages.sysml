// Two usages of one state definition each run their own copy of the braced
// entry, do, exit and effect blocks the definition declares, and a `terminate`
// in each copy ends that copy's block: both usages log the first assignment of
// every block and none of the trailing ones.
package Test {
	private import ScalarValues::*;

	attribute def Next;
	attribute def Go;

	state def Inner {
		attribute log : String = "";

		entry; then idle;
		state idle {
			entry { assign log := log + "e1;"; terminate; assign log := log + "e9;"; }
			do    { assign log := log + "d1;"; terminate; assign log := log + "d9;"; }
			exit  { assign log := log + "x1;"; terminate; assign log := log + "x9;"; }
		}
		state done;
		transition first idle accept Next
			do { assign log := log + "n1;"; terminate; assign log := log + "n9;"; }
			then done;
	}

	state def Machine {
		entry; then one;
		state one : Inner;
		state two : Inner;
		transition first one accept Go then two;
	}
}
