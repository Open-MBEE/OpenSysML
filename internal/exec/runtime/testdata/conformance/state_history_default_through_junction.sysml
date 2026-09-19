// A history reached before its composite state was ever left takes its default
// transition, here into a junction both of whose branches are enabled: exactly
// one is taken, which one is open, and the draw is recorded as a choice point
// at the junction like any other, so a sweep enumerates both entries.
package Test {
	state Machine {
		attribute route : Integer = 0;

		entry; then idle;
		state idle;
		state work {
			entry; then w1;
			state w1;
			state w2;
			history resume;
			junction split;

			transition first resume then split;
			transition first split do assign route := 1 then w1;
			transition first split do assign route := 2 then w2;
		}

		transition first idle accept Go then resume;
	}
}
