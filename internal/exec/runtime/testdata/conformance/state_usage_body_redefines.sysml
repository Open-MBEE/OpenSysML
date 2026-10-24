// A usage body adds to the content it inherits: the substate it declares joins
// the inherited ones. An attribute it declares with an inherited name does not
// redefine the inherited one (no `:>>`; the checker warns of the duplicate), so
// the entry actions `Base` declares read and write `Base::count` (KerML 1.0
// §8.2.3.5.3) and the usage's own `count` keeps its value beside it.
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
