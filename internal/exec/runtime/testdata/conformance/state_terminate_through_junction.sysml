// A junction whose only way on leads into a terminate action ends the machine as the
// compound transition completes: the source is exited, the segments' effects run in
// order, and the terminate is reached with no other state entered or exited.
package Test {
	private import ScalarValues::*;

	attribute def Check;

	state def Machine {
		attribute log : String = "";

		entry; then idle;
		state idle {
			exit { assign log := log + "idle(exit);"; }
		}
		junction relay;
		action stop terminate;

		transition first idle accept Check do { assign log := log + "check;"; } then relay;
		transition first relay do { assign log := log + "relay;"; } then stop;
	}
}
