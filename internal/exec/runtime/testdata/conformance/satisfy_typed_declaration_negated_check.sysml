// A negated satisfaction typed by its requirement definition holds when the
// definition's conditions evaluate to false on the `by` subject
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
		part fast : Lander {
			attribute :>> verticalSpeed = 2.5;
		}
		not satisfy requirement touchdown : Touchdown by fast;
	}
}
