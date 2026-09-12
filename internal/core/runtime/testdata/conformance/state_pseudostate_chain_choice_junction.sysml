// A junction beyond a choice on one route: the choice's guards are read once the
// incoming effect has run (`x == 1`), and the junction's at the point the route
// reaches it — as the branch into it is taken, before that branch's own effect
// runs, so `x == 1` still, not `x == 2`. The branch's effect runs after, in path order.
package Test {
	state Machine {
		attribute x : Integer = 0;

		entry; then idle;
		state idle;
		choice pick;
		junction split;
		state seen;
		state other;

		transition first idle do assign x := 1 then pick;
		transition first pick if x == 1 do assign x := 2 then split;
		transition first pick then other;
		transition first split if x == 2 then other;
		transition first split if x == 1 then seen;
		transition first seen then done;
		transition first other then done;
	}
}
