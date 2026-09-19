// A usage body adds to the content it inherits and redefines what it names
// again: its own attribute default replaces the inherited one, and the substate
// it declares joins the inherited ones.
package test {
	private import ScalarValues::*;

	state def Base {
		attribute count : Integer = 1;

		entry action start {
			assign count := count * 10;
		}
		entry; then working;
		state working {
			entry action step {
				assign count := count + 1;
			}
		}
	}

	state def Machine {
		entry; then u;
		state u : Base {
			attribute count : Integer = 100;
			state extra;
		}
	}
}
