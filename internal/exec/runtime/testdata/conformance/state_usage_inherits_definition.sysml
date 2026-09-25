// A state usage typed by a state definition inherits that definition's
// substates, initial transition, behaviors and attributes.
package test {
	private import ScalarValues::*;

	state def Inner {
		attribute hits : Integer = 0;

		entry; then i1;
		state i1 {
			entry action bump {
				assign hits := hits + 1;
			}
		}
	}

	state def Outer {
		entry; then nested;
		state nested : Inner;
	}
}
