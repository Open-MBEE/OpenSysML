// A chained target whose final feature is derived (KerML 1.0 §8.3.3.3
// Feature::isDerived) is refused: the object it reaches has that feature's
// values from its binding, not from the write.
package test {
	private import ScalarValues::*;

	part def Sensor {
		attribute raw : Integer default 4;
		derived attribute scaled : Integer = raw * 10;
	}

	action tune {
		part sensor : Sensor;

		first start;

		action force {
			assign sensor.scaled := 1;
		}

		done;

		succession first start then force;
		succession first force then done;
	}
}
