// A braced `do { … }` runs one statement at a time. Its first statement parks at
// an accept until Go arrives; the `terminate` reached after it ends the whole
// block, so the second accept the block would have parked at never begins, the
// do behavior is over, and the state completes to `t` rather than waiting on the
// timed transition. The machine keeps dispatching: `t` leaves for `u` on time.
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
		state stuck;

		transition first s then t;
		transition first s accept after 5 [SI::s] then stuck;
		transition first t accept after 1 [SI::s] then u;
	}
}
