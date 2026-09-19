// A machine whose substates are typed by a state definition, so the debugger
// steps through content the usages inherit rather than through leaf states.
package Typed {
	private import ScalarValues::*;

	state def Inner {
		attribute hits : Integer = 0;

		entry; then i1;
		state i1 {
			entry action bump {
				assign hits := hits + 1;
			}
		}
		accept after 5 [SI::s] then i2;
		state i2;
	}

	state def Machine {
		entry; then one;
		state one : Inner;
		state two : Inner;

		transition first one accept Swap then two;
	}
}
