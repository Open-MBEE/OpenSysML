// A part exhibiting two machines that both accept tap, the first's guard
// refusing it and the second's letting it through.
package Shared {
	private import ScalarValues::*;

	attribute def tap;

	part def Unit {
		attribute heaterArmed : Boolean = false;
		attribute fanArmed : Boolean = true;
		exhibit state heater {
			entry; then cold;
			state cold;
			transition first cold accept tap if heaterArmed then warm;
			state warm;
		}
		exhibit state fan {
			entry; then still;
			state still;
			transition first still accept tap if fanArmed then spinning;
			state spinning;
		}
	}

	part unit : Unit;
}
