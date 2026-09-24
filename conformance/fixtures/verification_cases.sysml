package Demo {
	private import ScalarValues::*;

	part def Widget {
		attribute m : Integer default = 0;
	}

	part good : Widget;

	requirement def Zeroed {
		subject w : Widget;
		require constraint { w.m == 0 }
	}

	requirement zeroed : Zeroed { subject w = good; }
	requirement bounded : Zeroed { subject w = good; }

	verification def ZeroCheck {
		subject w : Widget;
		objective { verify zeroed; }
		VerificationCases::PassIf(w.m == 0)
	}

	verification def BoundCheck {
		subject w : Widget;
		objective { verify bounded; }
		VerificationCases::PassIf(w.m == 1)
	}

	verification checkZero : ZeroCheck { subject w = good; }
	verification checkBound : BoundCheck { subject w = good; }

	part checks {
		assert satisfy zeroed by good;
		assert satisfy bounded by good;
	}
}
