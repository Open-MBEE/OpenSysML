// An assignment whose target is a feature chain writes the feature of the object
// the chain reaches: `s.reading` is written on the sensor, and the node after it
// reads that same feature back through the chain.
package test {
	private import ScalarValues::*;

	part def Sensor {
		attribute reading : Real = 0.0;
	}

	part s : Sensor;

	action write {
		attribute seen : Real = 0.0;

		first start;

		action set {
			assign s.reading := 4.5;
		}

		action read {
			assign seen := s.reading;
		}

		done;

		succession first start then set;
		succession first set then read;
		succession first read then done;
	}
}
