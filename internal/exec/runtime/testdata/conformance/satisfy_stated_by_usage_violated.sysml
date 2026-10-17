// A `satisfy r;` a usage states fails on an object of the usage whose values
// violate the requirement, the way `assert satisfy r by p` fails on `p`.
package test {
	part def Lander {
		attribute verticalSpeed;
	}

	requirement def Touchdown {
		subject lander : Lander;
		require constraint { lander.verticalSpeed <= 1.5 }
	}
	requirement touchdown : Touchdown;

	part rough : Lander {
		attribute :>> verticalSpeed = 2.4;
		satisfy touchdown;
	}
}
