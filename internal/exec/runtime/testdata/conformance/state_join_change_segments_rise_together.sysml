// One write raising the change conditions of every segment into a join is one
// occurrence enabling them all: the join fires, each segment's effect runs
// before the owner is left, in either region order, and the outgoing effect
// runs last.
package Test {
	private import ScalarValues::*;

	state Machine {
		attribute log : String = "";
		attribute level : Integer = 0;

		entry; then start;
		state start;
		state Work parallel {
			state a {
				entry; then a1;
				state a1;
				transition first a1 accept when level > 1 do assign log := log + "a " then sync;
			}
			state b {
				entry; then b1;
				state b1;
				transition first b1 accept when level > 2 do assign log := log + "b " then sync;
			}
		}
		join sync;
		transition first start do assign level := 3 then Work;
		transition first sync do assign log := log + "sync" then done;
	}
}
