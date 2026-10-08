// The first change-triggered segment arrives while the join's outgoing
// junction has no enabled branch; the later segment completes it after the
// condition falls and rises, when the junction's branch is enabled.
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
