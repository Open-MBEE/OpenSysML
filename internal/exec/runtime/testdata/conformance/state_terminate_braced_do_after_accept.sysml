// A braced `do { … }` runs one statement at a time. Its first statement parks at
// an accept until Go arrives; the `terminate` reached after it ends the whole
// block, so the second accept the block would have parked at never begins and
// the assignment after it never runs. With its do behavior over the state
// completes to `t`, and the machine keeps dispatching: the timed transition
// takes `t` to `u` a second later.
package Test {
	private import ScalarValues::*;
	private import SI::*;

	attribute def Go;

	state def Machine {
		attribute d : Integer = 0;

		entry; then s;

		state s {
			do {
				action wait { first start; then action w accept Go; then done; }
				assign d := 1;
				terminate;
				action again { first start; then action w accept Go; then done; }
				assign d := 9;
			}
		}
		state t;
		state u;

		transition first s then t;
		transition first t accept after 1 [SI::s] then u;
	}
}
