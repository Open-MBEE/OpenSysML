// A negated satisfaction's `result` is the requirement's own verdict, `false` on a
// subject violating it; the negation asserts that verdict false, never inverts it
// (Requirements.sysml notSatisfiedRequirementChecks :> negatedConstraintChecks :>
// Performances::falseEvaluations; SysML v2 §8.3.21.10).
package test {
	private import ScalarValues::*;

	part def Lander {
		attribute verticalSpeed : Real;
	}

	requirement def Touchdown {
		subject lander : Lander;
		require constraint { lander.verticalSpeed <= 1.5 }
	}

	part fast : Lander {
		attribute :>> verticalSpeed = 2.5;
	}

	not satisfy requirement touchdown : Touchdown by fast;
}
