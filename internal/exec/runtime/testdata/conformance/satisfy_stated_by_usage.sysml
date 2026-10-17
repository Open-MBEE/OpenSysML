// A `satisfy r;` a usage states is about the objects of that usage, so it is
// evaluated against one of them as `assert satisfy r by p` is against `p`.
package test {
	part def Lander {
		attribute verticalSpeed;
	}

	requirement def Touchdown {
		subject lander : Lander;
		require constraint { lander.verticalSpeed <= 1.5 }
	}
	requirement touchdown : Touchdown;

	part gentle : Lander {
		attribute :>> verticalSpeed = 0.5;
		satisfy touchdown;
	}
}
