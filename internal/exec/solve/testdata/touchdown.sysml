// A touchdown requirement over quantities: the assumption bounds the speed the
// requirement is written against, and the required condition compares two speeds.
// `5.4 [km/h]` is exactly `1.5 [m/s]`, which is what a translation must preserve.
package test {
	public import SI::*;
	public import ISQ::*;

	requirement def TouchdownRequirement {
		attribute actualVerticalSpeed : ISQSpaceTime::SpeedValue;
		attribute maxVerticalSpeed : ISQSpaceTime::SpeedValue;
		assume constraint {
			maxVerticalSpeed <= 5.4 [km/h]
		}
		require constraint {
			actualVerticalSpeed <= maxVerticalSpeed
		}
	}
}
