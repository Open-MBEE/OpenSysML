// Quantity slots as a client reads them: one written with a simple unit, one
// computed into a compound unit, one written in a scaled compound unit, one with
// an integer magnitude, and one inside a nested part.
package quantities {
	private import ScalarValues::*;

	part def Engine {
		attribute power : ISQ::PowerValue = 300.0 [SI::W];
	}

	part def Car {
		attribute mass : ISQ::MassValue = 5.0 [SI::kg];
		attribute plainMass : ScalarValues::Real = 2.0;
		attribute derivedSpeed = 10.0 [SI::m] / 2.0 [SI::s];
		attribute writtenSpeed = 5.4 [SI::km/SI::h];
		attribute length = 3 [SI::m];
		part engine : Engine;
	}
}
