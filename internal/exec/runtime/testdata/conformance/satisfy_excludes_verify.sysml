// An objective's `verify r;` is a requirement verification, not a satisfaction
// assertion: the satisfaction surfaces report the one `assert satisfy` here and
// leave the verification to the verification surfaces.
package test {
	private import ScalarValues::*;

	part def Lander {
		attribute verticalSpeed : Real;
	}
	part slow : Lander {
		attribute :>> verticalSpeed = 1.2;
	}

	requirement def Touchdown {
		subject lander : Lander;
		require constraint { lander.verticalSpeed <= 1.5 }
	}
	requirement touchdown : Touchdown;

	verification def TouchdownCheck {
		subject lander : Lander;
		objective { verify touchdown; }
		return verdict = VerificationCases::PassIf(lander.verticalSpeed <= 1.5);
	}

	part context {
		assert satisfy touchdown by slow;
	}
}
