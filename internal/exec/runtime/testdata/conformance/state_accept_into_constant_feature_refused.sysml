// An accepted payload bound into a feature the machine already declares writes
// that feature after the machine's occurrence is initialized, so a constant one
// (KerML 1.0 §8.3.3.3 Feature::isConstant) is refused and keeps its value.
package test {
	private import ScalarValues::*;

	attribute def Reading { attribute v : Integer; }

	state def Listener {
		constant attribute last : Reading;

		entry; then idle;

		state idle;

		transition first idle accept last : Reading then done;

		state done;
	}
}
