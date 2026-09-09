// An analysis case's objective subject defaults to the case's result
// (Cases::Case::obj), not to the case's subject: the same requirement-typed
// objective a verification case decides on its subject is left undecided by an
// analysis case returning a Real, whose result cannot be the Lander it needs.
package test {
	private import ScalarValues::*;

	part def Lander {
		attribute touchdownSpeed : Real;
	}
	part scout : Lander {
		attribute :>> touchdownSpeed = 1.25;
	}

	requirement def SoftLanding {
		subject lander : Lander;
		in attribute limit : Real default = 1.5;
		require constraint { lander.touchdownSpeed <= limit }
	}

	analysis def TouchdownMargin {
		subject lander : Lander;
		in attribute limit : Real = 1.5;
		objective : SoftLanding { in limit = limit; }
		return margin : Real = limit - lander.touchdownSpeed;
	}
	analysis marginOfScout : TouchdownMargin {
		subject lander = scout;
	}
}
