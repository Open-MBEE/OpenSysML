// A state's `do` and `exit` behaviors write through chains as its entry behavior
// does: leaving `go` leaves both features of the sensor written.
package test {
	private import ScalarValues::*;

	part def Sensor {
		attribute reading : Real = 0.0;
		attribute peak : Real = 0.0;
	}

	part def Rig {
		part s : Sensor;

		exhibit state run {
			entry; then go;

			state go {
				do action bump {
					assign s.reading := 3.5;
				}
				exit action stop {
					assign s.peak := 9.5;
				}
			}
			state settled;

			transition first go then settled;
		}
	}
}
