package test {
	part def Engine {
		attribute power = 300.0;
	}

	part def Vehicle {
		attribute mass = 1500.0;
		part engine : Engine;
	}
}
