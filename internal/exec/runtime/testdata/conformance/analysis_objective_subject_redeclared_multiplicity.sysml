// An objective redeclaring its subject without restating its multiplicity
// (`subject :>> pair;`) keeps the `Ship[2]` it redefines, so a two-Ship result
// defaults to it and the objective is decided.
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
		objective : PairLimit { subject :>> pair; }
		return picked : Ship[2] = (ship, heavy);
	}
	analysis pickPair : PickPair {
		subject ship = test::ship;
	}
}
