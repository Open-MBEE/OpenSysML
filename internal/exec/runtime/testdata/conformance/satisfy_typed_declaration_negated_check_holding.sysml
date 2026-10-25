// A negated satisfaction typed by its requirement definition is violated when the
// definition's conditions hold on the `by` subject
// (SysML v2 §7.21.4, §8.3.21.10; Requirements.sysml notSatisfiedRequirementChecks).
package test {
	private import ScalarValues::*;

	part def Lander {
		attribute verticalSpeed : Real;
	}

	requirement def Touchdown {
		subject lander : Lander;
		require constraint { lander.verticalSpeed <= 1.5 }
	}

	part mission {
		part gentle : Lander {
			attribute :>> verticalSpeed = 0.5;
		}
		not satisfy requirement touchdown : Touchdown by gentle;
	}
}
