// A satisfy usage a definition owns is about the owning object: `by g` names that
// object's `g` (SysML v2 §7.21.4, §8.4.17.3), so reading `result` through the object
// answers the check on that object's features.
package test {
	private import ScalarValues::*;

	part def Lander {
		attribute verticalSpeed : Real;
	}

	requirement def Touchdown {
		subject lander : Lander;
		require constraint { lander.verticalSpeed <= 1.5 }
	}

	part def Mission {
		part gentle : Lander {
			attribute :>> verticalSpeed = 0.5;
		}
		part hard : Lander {
			attribute :>> verticalSpeed = 2.5;
		}
		satisfy requirement soft : Touchdown by gentle;
		satisfy requirement rough : Touchdown by hard;
		attribute softVerdict : Boolean = soft.result;
		attribute roughVerdict : Boolean = rough.result;
	}
}
