// A machine whose only outgoing transition is a change trigger: the condition is
// made true by the state's own do behavior, and the transition out of `armed`
// waits on one that never holds.
package Debug {
	state Watch {
		attribute ready = false;
		attribute blocked = false;

		entry; then init;
		state init;
		state waiting {
			do {
				assign ready := true;
			}
		}
		state armed;

		succession first init then waiting;
		transition first waiting accept when ready then armed;
		transition first armed accept when blocked then done;
	}
}
