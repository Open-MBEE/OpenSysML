// A time trigger carrying a unit schedules the delay the unit measures:
// `accept after 5 [s]` waits five clock seconds.
package test {
	private import SI::*;

	state Machine {
		attribute log : Integer = 0;

		entry; then start;
		state start;
		state waiting;
		accept after 5 [s] then done;
		state done {
			entry { assign log := log * 10 + 1; }
		}

		succession first start then waiting;
	}
}
