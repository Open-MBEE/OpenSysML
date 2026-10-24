// A failing body decides the case's verdict on its own: the objective stating
// only `verify r;` stays satisfied, since it states no condition the run checks,
// and the verdict is the body's fail.
package test {
	private import ScalarValues::*;

	part def Lander {
		attribute verticalSpeed : Real;
	}
	part fast : Lander {
		attribute :>> verticalSpeed = 2.4;
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
	verification checkFast : TouchdownCheck {
		subject lander = fast;
	}
}
