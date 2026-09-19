// Materializing an object starts the machine its type exhibits: the object runs
// the machine itself, so its entry behavior writes the object's own attribute.
package test {
	private import ScalarValues::*;

	part def Counter {
		attribute count : Integer = 0;

		exhibit state counting {
			entry; then idle;

			state idle {
				entry action bump {
					assign count := count + 1;
				}
			}
		}
	}
}
