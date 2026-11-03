// A negated satisfaction's `result` is the requirement's own verdict, `true` on a
// subject meeting it; the negation asserts that verdict false, never inverts it
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

	part gentle : Lander {
		attribute :>> verticalSpeed = 0.5;
	}

	not satisfy requirement touchdown : Touchdown by gentle;
}
