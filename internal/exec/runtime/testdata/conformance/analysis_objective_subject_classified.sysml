package test {
	private import ScalarValues::*;

	part def Ship {
		attribute hullMass : Real;
	}
	part def Tanker :> Ship {
		attribute cargo : Real = 100.0;
	}
	part ship : Ship {
		attribute :>> hullMass = 1000.0;
	}

	requirement def LadenLimit {
		subject t : Tanker;
		require constraint { t.hullMass + t.cargo < 2000.0 }
	}

	analysis def Pick {
		subject s : Ship;
		objective : LadenLimit;
		return picked : Ship = s;
	}
	analysis light : Pick {
		subject s = ship;
	}
}
