// A transition effect writes through a chain as an entry behavior does: taking
// the transition writes the sensor the chain reaches.
package test {
	private import ScalarValues::*;

	part def Sensor {
		attribute reading : Real = 0.0;
	}

	part def Rig {
		part s : Sensor;

		exhibit state run {
			entry; then waiting;

			state waiting;
			state settled;

			transition first waiting do assign s.reading := 2.5 then settled;
		}
	}
}
