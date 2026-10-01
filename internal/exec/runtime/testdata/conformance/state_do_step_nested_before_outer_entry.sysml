// A do step of a state nested inside one region's parallel start state is
// drawn against the outer sibling region's entry as well as against its own
// sibling's: the nested front's units are units of the outer front.
package Test {
	state Machine {
		attribute log : String = "";

		entry; then idle;
		state idle;
		state work parallel {
			state left {
				entry; then l1;
				state l1 {
					entry { assign log := log + "l1(entry) "; }
				}
			}
			state right {
				entry; then r1;
				state r1 parallel {
					state ra {
						entry; then a1;
						state a1 {
							do { assign log := log + "did "; }
						}
					}
					state rb {
						entry; then b1;
						state b1 {
							entry { assign log := log + "b1(entry) "; }
						}
					}
				}
			}
		}
		transition first idle accept Go then work;
	}
	attribute def Go;
}
