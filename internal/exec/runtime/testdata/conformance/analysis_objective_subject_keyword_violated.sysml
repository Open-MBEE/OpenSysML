// The keyword-bound objective subject decides the verdict: a subject the
// requirement def's condition rejects leaves the objective not satisfied.
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

	analysis def MassCheck {
		subject ship : Ship;
		objective : MassLimit { subject = ship; }
		return r : Real = ship.hullMass;
	}
	analysis overweight : MassCheck {
		subject ship = heavy;
	}
}
