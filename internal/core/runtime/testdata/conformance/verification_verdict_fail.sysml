// The same body over a subject the condition is false of answers fail, computed
// by the library's own `PassIf` calculation rather than by the runtime.
package test {
	private import ScalarValues::*;

	part def Sensor {
		attribute reading : Integer = 3;
	}
	part drifted : Sensor;

	verification def ZeroCheck {
		subject sensor : Sensor;
		VerificationCases::PassIf(sensor.reading == 0)
	}

	verification checkDrifted : ZeroCheck {
		subject sensor = drifted;
	}
}
