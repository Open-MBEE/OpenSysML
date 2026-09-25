// An objective's subject binding and a body assertion read a completed action
// step's output (`weigh.m`), as the case's own outputs do; here both fail on a
// heavy ship, and the case still answers its result.
package test {
	private import ScalarValues::*;

	part def Ship {
		attribute hullMass : Real;
	}
	part heavy : Ship {
		attribute :>> hullMass = 5000.0;
	}

	action def Weigh {
		in s : Ship;
		out m : Real = s.hullMass;
	}

	requirement def MassCap {
		subject mass : Real;
		require constraint { mass < 2000.0 }
	}

	analysis def Weighed {
		subject ship : Ship;
		action weigh : Weigh { in s = ship; }
		objective : MassCap { subject = weigh.m; }
		assert constraint light { weigh.m < 2000.0 }
		return r : Real = weigh.m;
	}
	analysis weighedHeavy : Weighed {
		subject ship = heavy;
	}
}
