// `satisfy requirement sr : R by s;` declares a requirement usage typed by R, so
// its `result` is R's conditions evaluated with every subject R declares bound to
// the object `by` names, so a subject violating them reads `false`.
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
		attribute :>> verticalSpeed = 2.5;
	}

	satisfy requirement touchdown : Touchdown by gentle;
}
