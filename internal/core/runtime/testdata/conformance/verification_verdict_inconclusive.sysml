// A body that runs but binds no VerdictKind value decided nothing, which is the
// library's `inconclusive`.
package test {
	private import ScalarValues::*;

	part def Sensor {
		attribute reading : Integer = 0;
	}
	part zeroed : Sensor;

	verification def SilentCheck {
		subject sensor : Sensor;
		action measure {
			out reading : Integer = sensor.reading;
		}
	}

	verification checkSilently : SilentCheck {
		subject sensor = zeroed;
	}
}
