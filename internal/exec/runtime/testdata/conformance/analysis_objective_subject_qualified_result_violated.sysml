// The qualified implicit result (`MassCheck::result`) an objective's subject is
// bound to decides the verdict: a result the requirement def's condition rejects
// leaves the objective not satisfied. The body's `assert constraint` reads the
// result by the same qualified name, and is judged the same way.
package test {
	private import ScalarValues::*;

	part def Ship {
		attribute hullMass : Real;
	}
	part heavy : Ship {
		attribute :>> hullMass = 5000.0;
	}

	requirement def MassLimit {
		subject mass : Real;
		require constraint { mass < 2000.0 }
	}

	analysis def MassCheck {
		subject vessel : Ship;
		objective : MassLimit { subject = MassCheck::result; }
		assert constraint positive { MassCheck::result > 0.0 }
		vessel.hullMass
	}
	analysis overweight : MassCheck {
		subject vessel = heavy;
	}
}
