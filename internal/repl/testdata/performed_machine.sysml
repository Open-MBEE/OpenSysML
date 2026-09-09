// A part exhibiting a machine of its own beside a machine it can be made to
// perform, so a session over each of them has the other to be confused with.
package Two {
	part def Gauge {
		attribute count = 0;

		exhibit state modes {
			entry; then idle;
			state idle {
				entry action bump {
					assign count := count + 1;
				}
			}
		}
	}

	state def Check {
		entry; then checking;
		state checking;
		accept after 5 [SI::s] then checked;
		state checked;
	}

	part g : Gauge;
}
