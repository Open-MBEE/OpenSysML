// A requirement comparing quantities in commensurable but different units:
// 5.4 [km/h] is exactly 1.5 [m/s], so the condition holds at its boundary.
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
		attribute :>> actualVerticalSpeed = 5.4 [km/h];
		attribute :>> maxVerticalSpeed = 1.5 [m/s];
	}
}
