// A performed action reads the clock the run advances through the performer's
// `localClock.currentTime`: the Kernel gives every occurrence a localClock that
// defaults to the universal clock, whose currentTime is the shared clock in
// seconds. Stamping it before and after a timed wait leaves the elapsed time in
// the part, and a later read of the clock sees the same instant.
package test {
	private import ScalarValues::*;

	part def Timer {
		attribute started : Real = 0.0;
		attribute elapsed : Real = 0.0;

		action measure {
			action stamp {
				assign started := localClock.currentTime;
			}
			action wait accept after 2.5 [SI::s];
			action finish {
				assign elapsed := localClock.currentTime - started;
			}
			first stamp then wait;
			first wait then finish;
		}
	}

	part timer : Timer;

	action supervisor {
		attribute started : Real = -1.0;
		attribute elapsed : Real = -1.0;
		attribute now : Real = -1.0;

		first start;
		then action delay accept after 1.0 [SI::s];
		then perform action run ::> timer.measure;
		then action read {
			assign started := timer.started;
			assign elapsed := timer.elapsed;
			assign now := localClock.currentTime;
		}
		then done;
	}
}
