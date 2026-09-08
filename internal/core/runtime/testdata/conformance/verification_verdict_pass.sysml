// A verification case's body ending in the library's `PassIf` calculation
// answers the verdict that calculation computes: pass for a true condition.
package test {
	private import ScalarValues::*;

	part def Sensor {
		attribute reading : Integer = 0;
	}
	part zeroed : Sensor;

	requirement def ZeroRequirement {
		subject sensor : Sensor;
		require constraint { sensor.reading == 0 }
	}
	requirement zeroReading : ZeroRequirement;

	verification def ZeroCheck {
		subject sensor : Sensor;
		objective {
			verify zeroReading;
			require constraint { sensor.reading == 0 }
		}
		VerificationCases::PassIf(sensor.reading == 0)
	}

	verification checkZeroed : ZeroCheck {
		subject sensor = zeroed;
	}
}
