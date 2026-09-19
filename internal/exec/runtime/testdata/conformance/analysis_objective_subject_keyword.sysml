// An objective typed by a requirement def binds the def's subject the way a
// requirement usage does — here by keyword alone, `subject = ship;` — so its
// condition reads the case's subject through the def's own subject name.
package test {
	private import ScalarValues::*;

	part def Ship {
		attribute hullMass : Real;
	}
	part ship : Ship {
		attribute :>> hullMass = 1000.0;
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
	analysis light : MassCheck {
		subject ship = test::ship;
	}
}
