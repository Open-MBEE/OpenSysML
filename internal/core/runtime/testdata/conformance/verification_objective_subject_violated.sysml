// The objective decides against the verified subject: a lander touching down
// faster than the limit leaves the requirement unsatisfied while the body's
// PassIf, over the same subject, answers fail.
package test {
	private import ScalarValues::*;

	part def Lander {
		attribute touchdownSpeed : Real;
	}
	part scout : Lander {
		attribute :>> touchdownSpeed = 1.6;
	}

	requirement def SoftLanding {
		subject lander : Lander;
		in attribute limit : Real default = 1.5;
		require constraint { lander.touchdownSpeed <= limit }
	}

	verification def TouchdownCheck {
		subject lander : Lander;
		in attribute limit : Real = 1.5;
		objective : SoftLanding { subject :>> subj; in limit = limit; }
		VerificationCases::PassIf(lander.touchdownSpeed <= limit)
	}
	verification checkScout : TouchdownCheck {
		subject lander = scout;
	}
}
