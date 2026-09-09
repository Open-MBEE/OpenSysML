// A verification whose own subject nothing binds cannot run its body nor supply
// its objective's subject: the case's error verdict names the verification's
// subject, not the objective's.
package test {
	private import ScalarValues::*;

	part def Lander {
		attribute touchdownSpeed : Real;
	}

	requirement def SoftLanding {
		subject lander : Lander;
		in attribute limit : Real default = 1.5;
		require constraint { lander.touchdownSpeed <= limit }
	}

	verification def TouchdownCheck {
		subject lander : Lander;
		in attribute limit : Real = 1.5;
		objective : SoftLanding { in limit = limit; }
		VerificationCases::PassIf(lander.touchdownSpeed <= limit)
	}
	verification checkNothing : TouchdownCheck;
}
