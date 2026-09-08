// A body whose run cannot be carried out — nothing binds the case's subject —
// is the case's `error` verdict, carrying the reason.
package test {
	private import ScalarValues::*;

	part def Sensor {
		attribute reading : Integer = 0;
	}

	verification def ZeroCheck {
		subject sensor : Sensor;
		VerificationCases::PassIf(sensor.reading == 0)
	}
}
