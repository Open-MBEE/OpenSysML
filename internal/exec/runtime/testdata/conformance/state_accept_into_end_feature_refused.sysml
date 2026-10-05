// An accepted payload bound into a feature the machine already declares writes
// that feature after the machine's occurrence is initialized, so an end feature
// — constant as one that may vary in time (SysML 2.0 §8.4.2.2) — is refused and
// keeps its value.
package test {
	private import ScalarValues::*;

	attribute def Reading { attribute v : Integer; }

	state def Listener {
		end attribute last : Reading;

		entry; then idle;

		state idle;

		transition first idle accept last : Reading then done;

		state done;
	}
}
