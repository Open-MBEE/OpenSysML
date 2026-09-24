// Two objects of one type exhibit two machines: each has its own current state,
// its own event queue and its own attribute values, so a signal sent to one
// object's machine leaves its sibling's where it was.
package test {
	private import ScalarValues::*;

	part def Counter {
		attribute count : Integer = 0;

		exhibit state counting {
			entry; then idle;

			state idle;
			accept tick then ticked;

			state ticked {
				entry action bump {
					assign count := count + 1;
				}
			}
		}
	}
}
