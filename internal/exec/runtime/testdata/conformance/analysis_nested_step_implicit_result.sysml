// A nested analysis step returning its implicit result (a trailing expression,
// no `return` name) is read by the enclosing case as `inner.result`: in the
// objective's subject binding and in the case's own result alike.
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

	analysis def Mass {
		subject vessel : Ship;
		vessel.hullMass
	}

	analysis def Outer {
		subject vessel : Ship;
		analysis inner : Mass { subject vessel = vessel; }
		objective : MassLimit { subject = inner.result; }
		return total : Real = inner.result * 2.0;
	}
	analysis outer : Outer {
		subject vessel = heavy;
	}
}
