// `terminate` in a state's entry body ends that body alone: the statement
// after it is skipped while the state is entered and the machine goes on to
// take its transition.
package test {
	private import ScalarValues::*;

	attribute def Go;

	state def M {
		attribute n : Integer := 0;

		entry; then s1;
		state s1 {
			entry action { assign n := 1; terminate; assign n := 9; }
			exit action { assign n := n * 10 + 2; }
		}
		transition first s1 accept Go then s2;
		state s2;
	}
}
