// The result an unbound objective subject defaults to must also fit the
// subject's multiplicity: one Ship for a `Ship[2]` subject leaves the objective
// undecided, and the case's outputs still answered.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	part def Ship {
		attribute hullMass : Real;
	}
	part ship : Ship {
		attribute :>> hullMass = 1000.0;
	}

	requirement def PairLimit {
		subject pair : Ship[2];
		require constraint { pair->notEmpty() }
	}

	analysis def PickOne {
		subject ship : Ship;
		objective : PairLimit;
		return picked : Ship = ship;
	}
	analysis pickOne : PickOne {
		subject ship = test::ship;
	}
}
