// A junction ahead of a choice on one route: the junction's guards are read
// before the transition fires, against `x` as it was, and the choice's once the
// effects of the segments into it have run. `x == 0` routes through the junction
// and `x == 1` through the choice, both effects having run in path order.
package Test {
	state Machine {
		attribute x : Integer = 0;
		attribute y : Integer = 0;

		entry; then idle;
		state idle;
		junction split;
		choice pick;
		state seen;
		state other;

		transition first idle do assign x := 1 then split;
		transition first split if x == 0 do assign y := y + 1 then pick;
		transition first split then other;
		transition first pick if x == 1 do assign y := y + 10 then seen;
		transition first pick then other;
		transition first seen then done;
		transition first other then done;
	}
}
