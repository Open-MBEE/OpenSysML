// A part whose exhibited state usage references a state usage typed by a
// definition declared elsewhere, so the machine's body is two bindings away.
package Relay {
	private import ScalarValues::*;

	state def Blink {
		attribute ticks : Integer = 0;
		entry; then dark;
		state dark {
			entry action mark {
				assign ticks := ticks + 1;
			}
		}
		accept after 2 [SI::s] then lit;
		state lit;
		accept after 2 [SI::s] then dark;
	}

	part def Beacon {
		attribute ticks : Integer = 0;
		state spare : Blink;
		exhibit state active ::> spare;
	}

	part beacon : Beacon;
}
