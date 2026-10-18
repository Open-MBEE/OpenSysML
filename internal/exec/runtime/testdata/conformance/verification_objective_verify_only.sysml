// An objective stating only `verify r;` states what the case verifies, not a
// condition of its own: a requirement check over no constraints is true
// (Requirements::RequirementConstraintCheck), so the objective is satisfied and
// the case's verdict is its body's.
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
		objective gentle { verify touchdown; }
		return verdict = VerificationCases::PassIf(lander.verticalSpeed <= 1.5);
	}
	verification checkSlow : TouchdownCheck {
		subject lander = slow;
	}
}
