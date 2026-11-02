// A case binding its subject through a requirement's subject (`massLimit.vehicle`) reads the
// instantiated part the requirement's probe leaves standing, and decides the verdict its body selects.
package test {
	private import ScalarValues::Integer;
	private import VerificationCases::VerdictKind;

	part def Vehicle {
		attribute fuelMass : Integer;
	}

	part fullTankVehicle : Vehicle {
		attribute redefines fuelMass = -200;
	}

	requirement massLimit {
		subject vehicle : Vehicle = fullTankVehicle;
		require constraint { vehicle.fuelMass <= 2000 }
	}

	verification indirect {
		subject vehicle = massLimit.vehicle;
		objective { verify massLimit; }
		return verdict : VerdictKind =
			if vehicle.fuelMass >= 0 ? VerdictKind::pass else VerdictKind::inconclusive;
	}
}
