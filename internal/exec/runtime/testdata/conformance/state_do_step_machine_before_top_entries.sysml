// The machine's own do behavior begins before its regions are entered: its step is
// drawn against the top regions' entries on the front entering them, before,
// between or after them in either of their orders.
package Test {
	state Machine parallel {
		attribute log : String = "";

		do { assign log := log + "did "; }
		state left {
			entry; then l1;
			state l1 {
				entry { assign log := log + "l1(entry) "; }
			}
		}
		state right {
			entry; then r1;
			state r1 {
				entry { assign log := log + "r1(entry) "; }
			}
		}
	}
}
