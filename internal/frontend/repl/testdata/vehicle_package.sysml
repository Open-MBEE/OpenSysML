// A packaged model: nothing here is declared at the top level, so every REPL
// command that takes a name has to find a package member.
package Demo {
	part def Engine {
		attribute power = 300.0;
	}

	part def Vehicle {
		attribute mass = 1500.0;
		part engine : Engine;
	}

	calc add {
		in x;
		in y;
		x + y
	}

	constraint withinMassLimit {
		1500.0 < 2000.0
	}

	constraint overMassLimit {
		2500.0 < 2000.0
	}

	requirement SafeMass {
		require constraint { 1500.0 < 2000.0 }
	}
}
