// A chained target writes the final feature on the occurrence its chain reaches
// (Actions::AssignmentAction updates the accessedFeature of its target
// Occurrence), so that feature being constant (KerML 1.0 §8.3.3.3
// Feature::isConstant) refuses the write as a direct one is refused.
package test {
	private import ScalarValues::*;

	part def Sensor {
		constant attribute id : Integer = 1;
	}

	action tune {
		part sensor : Sensor;

		first start;

		action relabel {
			assign sensor.id := 2;
		}

		done;

		succession first start then relabel;
		succession first relabel then done;
	}
}
