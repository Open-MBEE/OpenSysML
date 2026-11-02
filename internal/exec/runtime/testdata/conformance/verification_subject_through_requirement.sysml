// A verification case binds its subject through a requirement's subject
// (`massLimit.vehicle`), itself bound to a package-level part. No object stands
// for the requirement, so the chain is read from the object the requirement
// would materialize under a probe; the part the subject names stood before the
// probe, so it is kept as read, and the case decides the explicit verdict its
// body selects — the same verdict the case bound to the part directly decides.
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
