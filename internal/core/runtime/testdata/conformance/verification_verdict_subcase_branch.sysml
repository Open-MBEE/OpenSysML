// A verification case whose flow decides between two subcases: only the branch
// the decision selects is performed, so only that subcase answered a verdict.
package test {
	private import ScalarValues::*;

	part def Sensor {
		attribute reading : Integer default = 0;
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
		attribute trusted : Boolean = true;
		action start;
		then decide route;
			if trusted then checkZeroed;
			if not trusted then checkDrifted;
		verification checkZeroed : ZeroCheck {
			subject sensor = test::zeroed;
		}
		verification checkDrifted : ZeroCheck {
			subject sensor = test::drifted;
		}
	}
}
