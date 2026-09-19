// Spacecraft mass model.
//
// Every comment, blank line and tab in this file is part of the edit contract:
// an edit rewrites the bytes of one value or name and nothing else.

package Demo {

	/*
	 * The spacecraft, with its mass properties.
	 */
	part def SC {
		// dry mass, as a quantity with a unit
		attribute unitMass : ISQ::MassValue default = 1000.0[SI::kg];

		attribute label : ScalarValues::String = "flight-1";
		attribute active : ScalarValues::Boolean = true;

		// no value yet: an edit adds one before the ';'
		attribute margin : ISQ::MassValue;

		attribute total : ISQ::MassValue = unitMass;

		part avionics {
			part board {
				attribute count : ScalarValues::Integer = 2;
			}
		}
	}

	part sc : SC {
		attribute redefines unitMass = 1200.0[SI::kg];
	}
}
