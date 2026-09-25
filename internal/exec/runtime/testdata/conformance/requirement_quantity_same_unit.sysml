// A requirement comparing quantities in the same unit, as the Open-MBEE
// lunar-lander model writes its touchdown requirement.
package test {
	public import SI::*;
	public import ISQ::*;

	requirement def TouchdownRequirement {
		attribute actualVerticalSpeed : ISQSpaceTime::SpeedValue;
		attribute maxVerticalSpeed : ISQSpaceTime::SpeedValue;
		require constraint {
			actualVerticalSpeed <= maxVerticalSpeed
		}
	}

	requirement touchdown : TouchdownRequirement {
		attribute :>> actualVerticalSpeed = 1.2 [m/s];
		attribute :>> maxVerticalSpeed = 1.5 [m/s];
	}
}
