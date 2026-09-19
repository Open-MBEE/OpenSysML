// A requirement declaring no subject of its own still receives the verification
// case's subject through the library's RequirementCheck::subj, and its
// constraint, reading only its own attribute, decides.
package test {
	private import ScalarValues::*;

	part def Lander {
		attribute touchdownSpeed : Real;
	}
	part scout : Lander {
		attribute :>> touchdownSpeed = 1.2;
	}

	requirement def SafeLimit {
		attribute limit : Real = 1.5;
		require constraint { limit > 1.0 }
	}

	verification def LimitCheck {
		subject lander : Lander;
		in attribute limit : Real = 1.5;
		objective : SafeLimit;
		VerificationCases::PassIf(lander.touchdownSpeed <= limit)
	}
	verification checkScout : LimitCheck {
		subject lander = scout;
	}
}
