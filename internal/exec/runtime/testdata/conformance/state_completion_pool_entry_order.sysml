// Two orthogonal regions whose initial states each complete at once, silently:
// neither performs anything, yet each entry generates a completion event, so the
// entry draw is observable through the pool's order — a completion event goes
// behind those already in the pool, so the completions dispatch in the order the
// entries were performed. The effects log that order; the two orders are the
// admissible set, and the entry draw is the run's one choice.
package Test {
	state Machine {
		attribute log : String = "";

		entry; then idle;
		state idle;
		state work parallel {
			state left {
				entry; then l1;
				state l1;
				state l2;
				transition first l1 do assign log := log + "l " then l2;
			}
			state right {
				entry; then r1;
				state r1;
				state r2;
				transition first r1 do assign log := log + "r " then r2;
			}
		}

		transition first idle accept Go then work;
	}
	attribute def Go;
}
