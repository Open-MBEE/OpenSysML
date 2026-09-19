// `<Case>::result` names that case's own run: an objective and an assertion of a
// heavy case reading a light sibling usage's result (`light::result`) judge the
// sibling's 1000.0, not the running case's 5000.0.
package test {
	private import ScalarValues::*;

	part def Ship {
		attribute hullMass : Real;
	}
	part ship : Ship {
		attribute :>> hullMass = 1000.0;
	}
	part heavy : Ship {
		attribute :>> hullMass = 5000.0;
	}

	requirement def MassLimit {
		subject mass : Real;
		require constraint { mass < 2000.0 }
	}

	analysis def Mass {
		subject vessel : Ship;
		vessel.hullMass
	}
	analysis light : Mass {
		subject vessel = test::ship;
	}
	analysis heavyBySibling : Mass {
		subject vessel = heavy;
		objective : MassLimit { subject = light::result; }
		assert constraint capped { light::result < 2000.0 }
	}
}
