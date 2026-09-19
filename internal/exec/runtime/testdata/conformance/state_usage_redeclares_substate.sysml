// A usage body redeclaring an inherited substate replaces it, and the inherited
// initial transition reaches the replacement rather than the dropped state.
package test {
	private import ScalarValues::*;

	state def Base {
		attribute count : Integer = 0;

		entry; then working;
		state working {
			entry action inherited {
				assign count := count + 1;
			}
		}
	}

	state def Machine {
		entry; then u;
		state u : Base {
			state working {
				entry action own {
					assign count := count + 500;
				}
			}
		}
	}
}
