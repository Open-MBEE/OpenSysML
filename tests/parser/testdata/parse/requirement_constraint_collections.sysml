package Collections {
	requirement def FuelRequirement {
		subject vehicle : Vehicle;
		assume constraint nonnegativeFuel :> assumptions { vehicle.fuelMass >= 0 }
		require constraint fuelLimit :> constraints { vehicle.fuelMass <= 2000 }
	}

	requirement fullTankRequirement : FuelRequirement {
		subject vehicle : Vehicle = vehicleUnderTest;
		assume constraint fullTank :> assumptions { vehicle.fuelMass == vehicle.fuelFullMass }
	}

	verification check {
		objective { verify fullTankRequirement; }
		return verdict : VerdictKind =
			if allTrue(fullTankRequirement.assumptions()) ? PassIf(fullTankRequirement()) else VerdictKind::inconclusive;
	}
}
