// A state entry behavior writes through a chain and the machine's own guard reads
// that write back: `go` is only left because the sensor now reads 4.5.
package test {
	private import ScalarValues::*;

	part def Sensor {
		attribute reading : Real = 0.0;
	}

	part def Rig {
		part s : Sensor;

		exhibit state run {
			entry; then go;

			state go {
				entry action set {
					assign s.reading := 4.5;
				}
			}
			state tripped;

			transition first go if s.reading > 1.0 then tripped;
		}
	}
}
