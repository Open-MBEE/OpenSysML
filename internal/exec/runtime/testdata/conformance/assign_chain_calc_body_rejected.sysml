// A calculation computes a value and writes nothing outside its own body, so a
// chained target is rejected rather than performed.
package test {
	private import ScalarValues::*;

	part def Sensor {
		attribute reading : Real = 0.0;
	}

	part s : Sensor;

	calc def Reset {
		assign s.reading := 1.0;
		2.0
	}
}
