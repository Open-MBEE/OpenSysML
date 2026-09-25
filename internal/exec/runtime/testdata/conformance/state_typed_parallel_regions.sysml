// A typed usage in a parallel body is an orthogonal region entered at the
// initial state its definition declares.
package test {
	private import ScalarValues::*;

	state def Region {
		attribute ticks : Integer = 0;

		entry; then r1;
		state r1 {
			entry action tick {
				assign ticks := ticks + 1;
			}
		}
	}

	state def Machine {
		entry; then running;
		state running parallel {
			state a : Region;
			state b : Region;
		}
	}
}
