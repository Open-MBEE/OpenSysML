// A guard of a machine an object exhibits reads the object's feature values, so
// it sees what the machine's own entry body wrote rather than the declared
// default: `armed` is only left because `count` is no longer 0.
package test {
	private import ScalarValues::*;

	part def Monitor {
		attribute count : Integer = 0;

		exhibit state modes {
			entry; then armed;

			state armed {
				entry action bump {
					assign count := count + 1;
				}
			}
			state tripped;

			transition arm_to_trip first armed if count > 0 then tripped;
		}
	}
}
