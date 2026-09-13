// Two choices on one route: each is resolved on arrival, against the data as the
// effects of the segments into it left it. The first reads `x == 1` written by the
// incoming effect; the second reads `x == 2` written by the first's branch.
package Test {
	state Machine {
		attribute x : Integer = 0;

		entry; then idle;
		state idle;
		choice pickA;
		choice pickB;
		state seen;
		state other;

		transition first idle do assign x := 1 then pickA;
		transition first pickA if x == 1 do assign x := 2 then pickB;
		transition first pickA then other;
		transition first pickB if x == 2 then seen;
		transition first pickB then other;
		transition first seen then done;
		transition first other then done;
	}
}
