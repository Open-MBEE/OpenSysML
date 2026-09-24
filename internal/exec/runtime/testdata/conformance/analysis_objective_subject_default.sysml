// An objective subject nothing binds defaults to the case's result, as the
// library declares (Cases::Case::obj: `subject subj default Case::result`): the
// objective checks the Ship the case returns.
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

	analysis def PickShip {
		subject ship : Ship;
		objective : MassLimit;
		return picked : Ship = ship;
	}
	analysis picked : PickShip {
		subject ship = heavy;
	}
}
