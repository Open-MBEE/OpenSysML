// The notation binding behaviors to a type's objects: a machine the type
// exhibits, one it exhibits by name, and actions it performs.
package test {

	state def Modes;

	action def Report;

	part def Monitor {

		attribute count : Integer = 0;

		exhibit state tracking {
			entry; then idle;
			state idle {
				entry action bump {
					assign count := count + 1;
				}
			}
		}

		exhibit state named : Modes;

		exhibit state documented : Modes {
			doc /* the modes it is in */
		}

		perform action report : Report;

		perform action sample {
			assign count := count + 1;
		}
	}
}
