// A subject bound by an expression must hold as many values as it declares, a
// redeclaration without a multiplicity keeping the redefined subject's: one Ship
// for a `Ship[2]` subject leaves the objective undecided.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	part def Ship {
		attribute hullMass : Real;
	}
	part ship : Ship {
		attribute :>> hullMass = 1000.0;
	}
	part heavy : Ship {
		attribute :>> hullMass = 5000.0;
	}

	requirement def PairLimit {
		subject pair : Ship[2];
		require constraint { pair->notEmpty() }
	}

	analysis def PickOne {
		subject ship : Ship;
		objective : PairLimit { subject :>> pair = ship; }
		return picked : Ship = ship;
	}
	analysis pickOne : PickOne {
		subject ship = test::ship;
	}
}
