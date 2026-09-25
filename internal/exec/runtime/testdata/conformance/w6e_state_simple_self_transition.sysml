// A transition back to its own simple state is external, as one out of a
// composite state is: the state is exited, the effect runs, and the state is
// entered again, so its exit and entry actions both run once per firing.
package Test {
	state Machine {
		attribute log : Integer = 0;

		entry; then s;
		state s {
			entry { assign log := log * 10 + 1; }
			exit { assign log := log * 10 + 2; }
		}

		transition first s accept again do assign log := log * 10 + 9 then s;
	}
}
