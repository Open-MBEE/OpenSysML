// `terminate` written in a braced `entry { … }`, `do { … }`, `exit { … }` or
// transition `do { … }` block ends the whole block, not just the statement it
// stands in: the assignments after it never run, the state stays active, its do
// behavior still starts after the entry block ends, and the transition whose exit
// and effect blocks end early still completes to its target.
package test {
	private import ScalarValues::*;
	private import SI::*;

	state def Machine {
		attribute e : Integer = 0;
		attribute d : Integer = 0;
		attribute x : Integer = 0;
		attribute n : Integer = 0;

		entry; then s;

		state s {
			entry { assign e := 1; then terminate; then assign e := 9; }
			do    { assign d := 1; then terminate; then assign d := 9; }
			exit  { assign x := 1; then terminate; then assign x := 9; }
		}
		state t;

		transition first s accept after 1 [SI::s]
			do { assign n := 1; then terminate; then assign n := 9; }
			then t;
	}
}
