// The Time library's universal clock reads the shared clock as a
// TimeInstantValue: a point on the clock's own time scale, whose magnitude is
// the seconds since the run began — the instant `accept at` waits for. The read
// stores into a TimeInstantValue, two reads differ by a DurationValue, an
// instant derived from one is waited for, and two instants order.
package test {
	private import SI::*;
	private import Time::*;
	private import ScalarValues::*;

	action stopwatch {
		attribute started : TimeInstantValue;
		attribute due : TimeInstantValue;
		attribute elapsed : DurationValue;
		attribute later : Boolean = false;

		first start;
		then action stamp assign started := Time::universalClock.currentTime;
		then action wait accept after 2 [s];
		then action schedule assign due := Time::universalClock.currentTime + 3 [s];
		then action hold accept at due;
		then action measure {
			assign elapsed := Time::universalClock.currentTime - started;
			assign later := Time::universalClock.currentTime > started;
		}
		then done;
	}
}
