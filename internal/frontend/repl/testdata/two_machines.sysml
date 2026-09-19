// A part exhibiting two machines, so a debugger attached to the second has a
// first to be confused with.
package Pair {
	part def Gauge {
		exhibit state power {
			entry; then off;
			state off;
			accept after 5 [SI::s] then on;
			state on;
		}

		exhibit state link {
			entry; then idle;
			state idle;
			accept after 3 [SI::s] then busy;
			state busy;
		}
	}

	part gauge : Gauge;
}
