// A deep history restores two regions' recorded leaves, each nested in a
// composite state and each completing at once when restored: its completion
// transition is guarded on a flag the pause raised, so the completion event
// generated at the first entry was dropped and the one generated at the restore
// fires. The composites' and the leaves' entries perform nothing, yet each leaf's
// entry generates a completion event, so the restore's entry draw decides the
// pool's order and the effects log it; both orders are admissible.
package Test {
	state Machine {
		attribute log : String = "";
		attribute resumed : Boolean = false;

		entry; then work;
		state work parallel {
			deep history resume;
			state left {
				entry; then l1;
				state l1;
				state lc {
					entry; then lc1;
					state lc1;
					state lc2;
					transition first lc1 then lc2;
					transition first lc2 if resumed do assign log := log + "l " then lc3;
				}
				state lc3;
				transition first l1 accept Go then lc;
			}
			state right {
				entry; then r1;
				state r1;
				state rc {
					entry; then rc1;
					state rc1;
					state rc2;
					transition first rc1 then rc2;
					transition first rc2 if resumed do assign log := log + "r " then rc3;
				}
				state rc3;
				transition first r1 accept Go then rc;
			}
		}
		state paused {
			entry { assign resumed := true; }
		}

		transition first work accept Pause then paused;
		transition first paused accept Resume then resume;
	}
	attribute def Go;
	attribute def Pause;
	attribute def Resume;
}
