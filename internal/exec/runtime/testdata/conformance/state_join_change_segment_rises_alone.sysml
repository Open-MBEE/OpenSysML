// A change condition rising is one occurrence: a change-triggered segment whose
// condition rose holds the join while the other segment's condition is still
// false, and a later rise of the other condition is a different occurrence, so
// the join never fires and no segment's effect runs.
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
				transition first c1 accept Bump do assign level := 3 then c1;
			}
		}
		join sync;
		transition first start do assign level := 2 then Work;
		transition first sync do assign log := log + "sync" then done;
	}
}
