// An objective binding its subject to the case's implicit result by qualified
// name (`subject = MassCheck::result;`, as the OMG pilot's analysis example
// writes it) reads the value the run's trailing expression yields: satisfied.
package test {
	private import ScalarValues::*;

	part def Ship {
		attribute hullMass : Real;
	}
	part ship : Ship {
		attribute :>> hullMass = 1000.0;
	}

	requirement def MassLimit {
		subject mass : Real;
		require constraint { mass < 2000.0 }
	}

	analysis def MassCheck {
		subject vessel : Ship;
		objective : MassLimit { subject = MassCheck::result; }
		vessel.hullMass
	}
	analysis light : MassCheck {
		subject vessel = ship;
	}
}
