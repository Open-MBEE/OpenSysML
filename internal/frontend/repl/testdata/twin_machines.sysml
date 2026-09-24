// A part exhibiting two machines, so a session over the second has a first it
// could be confused with when the object's machines are restarted.
package Twins {
	attribute def heat;
	attribute def spin;

	state def Heater {
		entry; then cold;
		state cold;
		transition first cold accept heat then warm;
		state warm;
	}

	state def Fan {
		entry; then still;
		state still;
		transition first still accept spin then spinning;
		state spinning;
	}

	part def Unit {
		exhibit state heater : Heater;
		exhibit state fan : Fan;
	}

	part unit : Unit;
}
