// A verification case's objective takes the case's subject as its own
// (VerificationCases::VerificationCase::obj binds `subj = VerificationCase::subj`),
// so an objective typed by a requirement whose subject bears another name
// (`lander`) evaluates that requirement on the verified lander and decides.
package test {
	private import ScalarValues::*;

	part def Lander {
		attribute touchdownSpeed : Real;
	}
	part scout : Lander {
		attribute :>> touchdownSpeed = 1.2;
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
