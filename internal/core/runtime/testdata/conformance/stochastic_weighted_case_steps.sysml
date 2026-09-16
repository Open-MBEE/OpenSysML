// A verification case whose flow weights the decision among its subcases: the
// weights are read as an action's are, so unseeded under `declared` the most
// probable branch is performed; read unweighted, the last unguarded succession
// would be taken as the else branch instead.
package test {
	private import ScalarValues::*;
	private import Stochastic::*;

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
		action start;
		then decide route;
		first route then checkZeroed { @Probability { p = 0.8; } }
		first route then checkDrifted { @Probability { p = 0.2; } }
		verification checkZeroed : ZeroCheck {
			subject sensor = test::zeroed;
		}
		verification checkDrifted : ZeroCheck {
			subject sensor = test::drifted;
		}
	}
}
