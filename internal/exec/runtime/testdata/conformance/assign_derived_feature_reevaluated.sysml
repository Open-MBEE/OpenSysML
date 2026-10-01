// A derived feature's values are determined by the model (KerML 1.0 §8.3.3.3
// Feature::isDerived), here by binding it to an expression over another
// feature: writing that other feature is allowed, and the derived feature reads
// the value the binding gives it afterwards.
package test {
	private import ScalarValues::*;

	part def Sensor {
		attribute raw : Integer default 4;
		derived attribute scaled : Integer = raw * 10;
	}

	action tune {
		part sensor : Sensor;
		attribute before : Integer = 0;
		attribute after : Integer = 0;

		first start;

		action step {
			assign before := sensor.scaled;
			assign sensor.raw := 5;
			assign after := sensor.scaled;
		}

		done;

		succession first start then step;
		succession first step then done;
	}
}
