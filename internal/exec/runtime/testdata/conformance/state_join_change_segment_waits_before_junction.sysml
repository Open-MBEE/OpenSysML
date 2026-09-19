// A change-triggered segment whose condition rose alone holds the join without
// looking past it: the junction the join leads to has no branch enabled while
// only `a`'s condition holds, and that is not a dead end since the join does
// not fire. Once the level falls and rises again for both segments at once, the
// join fires and the junction's branch is enabled.
package Test {
	private import ScalarValues::*;

	attribute def Bump;

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
			state c {
				entry; then c1;
				state c1;
				state c2;
				transition first c1 accept Bump do assign level := 0 then c2;
				transition first c2 accept Bump do assign level := 3 then c2;
			}
		}
		join sync;
		junction onward;
		transition first start do assign level := 2 then Work;
		transition first sync do assign log := log + "sync " then onward;
		transition first onward if level > 2 do assign log := log + "onward" then done;
	}
}
