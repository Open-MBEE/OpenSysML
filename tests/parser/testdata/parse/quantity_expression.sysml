// Quantity expressions and the sequence index, which share a node but not a
// syntax: `[unit]` pairs a magnitude with a measurement reference, `#(i)` indexes.
package quantities {
	private import SI::*;

	attribute maxVerticalSpeed = 1.5 [m/s];
	attribute descentDistance = 3.0 [km];
	attribute perAreaRate = 2.0 [kg / m ** 2];
	attribute count = 1 [1];
	attribute third = speeds#(3);

	constraint withinLimit {
		1.5 [m/s] <= 5.4 [km/h]
	}
}
