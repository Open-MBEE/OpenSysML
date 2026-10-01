package Demo {
	private import ScalarValues::*;

	part def Probe {
		attribute x : Real;
	}

	part probe : Probe {
		attribute :>> x = 1.0;
	}

	assert constraint sq { probe.x ** 2.0 >= 0.0 }
	assert constraint truth { probe.x == probe.x }
}
