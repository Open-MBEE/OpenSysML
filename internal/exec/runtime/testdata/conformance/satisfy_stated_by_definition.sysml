// A `satisfy r;` a definition states is about the objects of that definition,
// so with none to evaluate it against the assertion asks for one rather than
// reading the requirement's subject from nothing.
package test {
	part def Lander {
		attribute verticalSpeed;
	}

	requirement def Touchdown {
		subject lander : Lander;
		require constraint { lander.verticalSpeed <= 1.5 }
	}
	requirement touchdown : Touchdown;

	part def SafeLander :> Lander {
		satisfy touchdown;
	}
}
