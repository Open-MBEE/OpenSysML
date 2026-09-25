// A state machine's attribute defaults, guards and time-event durations are
// evaluated in the scope they were written in, so the document's imports and its
// package's members are visible to them: the guard compares 1.5 [m/s] with the
// package-level 4.0 [km/h] and converts, and the time event's delay reads a
// package-level attribute. A state's entry action and a transition's effect
// resolve in their own scopes too, so the units they write resolve as well.
package test {
	private import ScalarValues::*;
	private import SI::*;
	private import ISQ::*;

	attribute pkgDelay : ISQBase::DurationValue = 1.0 [s];
	attribute pkgLimit : ISQSpaceTime::SpeedValue = 4.0 [km/h];

	state descent {
		attribute speed : ISQSpaceTime::SpeedValue = 1.5 [m/s];
		attribute margin : ISQSpaceTime::SpeedValue = 2.0 [m/s] - 0.5 [m/s];
		attribute verdict : String = "unknown";
		attribute touchdown : ISQSpaceTime::SpeedValue = 0.0 [m/s];
		attribute logged : ISQSpaceTime::SpeedValue = 0.0 [m/s];

		entry; then start;
		state start;
		state falling;
		state slow {
			entry { assign verdict := "slow"; }
		}
		state fast {
			entry { assign verdict := "fast"; assign touchdown := speed + 0.5 [m/s]; }
		}

		succession first start then falling;

		transition first falling if speed <= pkgLimit then slow;
		transition first falling then fast;

		transition first slow accept after pkgDelay then done;
		transition first fast accept after pkgDelay do { assign logged := touchdown - 0.25 [m/s]; } then done;
	}
}
