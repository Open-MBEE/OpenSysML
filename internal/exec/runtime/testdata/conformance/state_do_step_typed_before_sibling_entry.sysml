// The do behavior is a typed action of two steps whose inout writes back as it
// ends, started as its state is entered: each step is one move drawn against
// the sibling region's remaining entry while it is left, so the entry's write
// falls before the write-back, which overwrites it, or after it.
package Test {
	private import ScalarValues::*;

	state Machine {
		attribute count : Integer = 0;

		action def Work {
			inout n : Integer;
			first start;
			then action a assign n := n + 1;
			then action b assign n := n + 10;
			then done;
		}

		entry; then idle;
		state idle;
		state work parallel {
			state left {
				entry; then l1;
				state l1 {
					do action job : Work { inout n = count; }
				}
			}
			state right {
				entry; then r1;
				state r1 {
					entry { assign count := count + 100; }
				}
			}
		}
		transition first idle accept Go then work;
	}
	attribute def Go;
}
