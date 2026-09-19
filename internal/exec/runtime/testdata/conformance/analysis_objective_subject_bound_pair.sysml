// Two Ships bound by an expression to a `Ship[2]` subject satisfy its multiplicity.
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

	analysis def PickPair {
		subject ship : Ship;
		objective : PairLimit { subject = (ship, heavy); }
		return picked : Ship = ship;
	}
	analysis pickPair : PickPair {
		subject ship = test::ship;
	}
}
