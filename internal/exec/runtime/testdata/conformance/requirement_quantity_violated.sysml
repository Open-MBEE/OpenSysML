// A requirement violated by a quantity that exceeds its limit: the magnitudes
// are compared in commensurable units, not as bare numbers.
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
		attribute :>> actualVerticalSpeed = 7.2 [km/h];
		attribute :>> maxVerticalSpeed = 1.5 [m/s];
	}
}
