// A region's do behavior and the completions of two sibling regions due at one
// instant: as work is entered, l1's do behavior is due and m1's and r1's
// completion events are in the pool, queued in the order the states completed.
// Under check and explore each instant both are due draws the do step against
// the completion at the head of the pool, so the do step falls before, between
// or after the two completion effects; the fixed policies run the do round to
// its end and dispatch after it.
package Test {
	private import ScalarValues::*;

	state Machine {
		attribute log : String = "";

		entry; then work;
		state work parallel {
			state left {
				entry; then l1;
				state l1 {
					do { assign log := log + "did "; }
				}
			}
			state middle {
				entry; then m1;
				state m1;
				transition first m1 do assign log := log + "m " then m2;
				state m2;
			}
			state right {
				entry; then r1;
				state r1;
				transition first r1 do assign log := log + "r " then r2;
				state r2;
			}
		}
	}
}
