// A satisfaction declared with its requirement definition as type
// (`satisfy requirement sr : R by s;`) is checked against R's conditions
// (SysML v2 §7.21.4, §8.4.17.3), as the `satisfy r by s;` reference form is.
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
		satisfy requirement touchdown : Touchdown by gentle;
	}
}
