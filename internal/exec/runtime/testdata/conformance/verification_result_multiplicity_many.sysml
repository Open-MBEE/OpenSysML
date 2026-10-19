// A verification case whose verdict result declares [0..*] holds the verdict as
// a one-element sequence, the shape its multiplicity gives it, and the verdict
// is read from that sequence's one element: the case passes rather than deciding nothing.
package test {
	private import ScalarValues::*;
	private import VerificationCases::*;

	part def Sensor {
		attribute reading : Integer default = 0;
	}
	part zeroed : Sensor;

	verification def ZeroCheck {
		subject sensor : Sensor;
		return verdict : VerdictKind[0..*] = PassIf(sensor.reading == 0);
	}

	verification checkZeroed : ZeroCheck {
		subject sensor = zeroed;
	}
}
