// PSSM Terminate 002's shape: two regions, the first's state with a do activity
// of two segments around an accept, the second's state completing into a
// terminate. The first segment is drawn against the sibling's entry and then
// against the completion's dispatch; the terminate ends the machine before the
// accept is fed, so the second segment never runs.
package Test {
	state Machine {
		attribute log : String = "";

		entry; then idle;
		state idle;
		state work parallel {
			entry { assign log := log + "work(entry) "; }
			state left {
				entry; then l1;
				state l1 {
					entry { assign log := log + "l1(entry) "; }
					do action {
						first start;
						then action partI assign log := log + "l1(partI) ";
						then action wait accept Continue;
						then action partII assign log := log + "l1(partII) ";
						then done;
					}
				}
			}
			state right {
				entry; then r1;
				state r1 {
					entry { assign log := log + "r1(entry) "; }
				}
				action stop terminate;
				transition first r1 then stop;
			}
		}
		transition first idle accept Go then work;
	}
	attribute def Go;
	attribute def Continue;
}
