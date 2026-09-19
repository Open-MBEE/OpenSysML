// A packaged model whose attributes are derived from other features, so
// instantiating it is only useful if defaults are evaluated against the
// instance rather than constant-folded.
package Derived {
	part def Engine {
		attribute power = 300.0;
		attribute derated = power * 0.9;
	}

	part def Vehicle {
		attribute mass = 1500.0;
		attribute doubled = mass * 2.0;
		part engine : Engine;
		attribute total = mass + engine.derated;

		constraint massOK {
			mass < 2000.0
		}

		requirement lightEnough {
			require constraint { mass < 2000.0 }
		}
	}

	part def Heavy {
		attribute mass = 2500.0;

		constraint massOK {
			mass < 2000.0
		}

		requirement lightEnough {
			require constraint { mass < 2000.0 }
		}
	}
}
