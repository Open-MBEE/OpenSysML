// A choice reads its guards once the transition into it has run its effect, so a
// guard over a value the effect writes sees the written value: `x == 1` holds and
// `seen` is entered, with the branch's own effect run after the incoming one.
package Test {
	state Machine {
		attribute x : Integer = 0;
		attribute y : Integer = 0;

		entry; then idle;
		state idle;
		choice pick;
		state seen;
		state other;

		transition first idle do assign x := 1 then pick;
		transition first pick if x == 1 do assign y := 7 then seen;
		transition first pick do assign y := 3 then other;
		transition first seen then done;
		transition first other then done;
	}
}
