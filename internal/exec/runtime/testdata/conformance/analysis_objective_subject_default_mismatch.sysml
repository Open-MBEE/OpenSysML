// The result an unbound objective subject defaults to must be of the subject's
// type: a case returning a Real for a Ship subject leaves the objective
// undecided, and the case's outputs still answered.
package test {
	private import ScalarValues::*;

	part def Ship {
		attribute hullMass : Real;
	}
	part heavy : Ship {
		attribute :>> hullMass = 5000.0;
	}

	requirement def MassLimit {
		subject s : Ship;
		attribute limit : Real = 2000.0;
		require constraint { s.hullMass < limit }
	}

	analysis def MassOnly {
		subject ship : Ship;
		objective : MassLimit;
		return r : Real = ship.hullMass;
	}
	analysis massOnly : MassOnly {
		subject ship = heavy;
	}
}
