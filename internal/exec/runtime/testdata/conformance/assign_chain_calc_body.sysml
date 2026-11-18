// A calculation may have side effects (KerML 7.4.8.1), so a chained target in its
// body is written on the object the chain reaches, and the result reads it.
package test {
	private import ScalarValues::*;

	part def Sensor {
		attribute reading : Real := 0.0;
	}

	part s : Sensor;

	calc def Reset {
		assign s.reading := 1.5;
		return : Real[1] = s.reading + 2.0;
	}
}
