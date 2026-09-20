// A shallow history restores two regions' recorded states, each of which
// completes at once when restored: its completion transition is guarded on a
// flag the pause raised, so the completion event generated at the first entry
// was dropped and the one generated at the restore fires. The entries perform
// nothing, yet each generates a completion event, so the restore's entry draw
// decides the pool's order and the effects log it; both orders are admissible.
package Test {
	state Machine {
		attribute log : String = "";
		attribute resumed : Boolean = false;

		entry; then work;
		state work parallel {
			shallow history resume;
			state left {
				entry; then l1;
				state l1;
				state l2;
				state l3;
				transition first l1 accept Go then l2;
				transition first l2 if resumed do assign log := log + "l " then l3;
			}
			state right {
				entry; then r1;
				state r1;
				state r2;
				state r3;
				transition first r1 accept Go then r2;
				transition first r2 if resumed do assign log := log + "r " then r3;
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
