// Two transitions into a join fire when it does, in an order the run draws.
// Each source has another transition of its own that writes the log, the state
// owning the join writes it on exit, and the join's way out writes it too: none
// of that is the pending segments' own doing — the unrelated transitions do not
// fire, and the owner's exit and the way out follow whichever segment fires
// first. The segments themselves write apart features, so the order drawn is
// open but changes no value, and nothing is marked order-dependent.
package Test {
	state Machine {
		attribute a : Integer = 0;
		attribute b : Integer = 0;
		attribute log : Integer = 0;

		entry; then Owner;
		state Owner parallel {
			exit assign log := log + 1;
			state left {
				entry; then l1;
				state l1;
				transition first l1 accept Go do assign a := 1 then sync;
				transition first l1 accept Other do assign log := 10 then l1;
			}
			state right {
				entry; then r1;
				state r1;
				transition first r1 accept Go do assign b := 2 then sync;
				transition first r1 accept Other do assign log := 20 then r1;
			}
			join sync;
			transition first sync do assign log := log + 100 then done;
		}
		state done;
	}
}
