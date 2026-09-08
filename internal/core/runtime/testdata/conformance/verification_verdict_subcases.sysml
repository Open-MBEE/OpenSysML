// A verification case performing verification subcases: the library states no
// roll-up of a subcase's verdict into its parent's, so each subcase is reported
// on its own and the parent reports what its own body bound.
package test {
	private import ScalarValues::*;

	part def Sensor {
		attribute reading : Integer = 0;
	}
	part zeroed : Sensor;
	part drifted : Sensor {
		attribute :>> reading = 4;
	}

	verification def ZeroCheck {
		subject sensor : Sensor;
		VerificationCases::PassIf(sensor.reading == 0)
	}

	verification plan {
		subject sensor = zeroed;
		verification checkZeroed : ZeroCheck {
			subject sensor = test::zeroed;
		}
		verification checkDrifted : ZeroCheck {
			subject sensor = test::drifted;
		}
	}
}
