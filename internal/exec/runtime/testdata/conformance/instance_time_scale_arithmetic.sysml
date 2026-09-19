// A time instant on Time::UTC is a point on a TimeScale, an IntervalScale whose
// unit is the second. Adding a duration moves the instant along the scale and
// answers on it, in whatever duration unit the offset was given; subtracting
// two instants answers their separation in the scale's unit, s. An instant has
// no double, no product with a duration, no negative and no sum with another
// instant, and instants do not fold under sum. UTC states no anchor on another
// reference, so an instant compares with an instant on UTC, not with a bare
// duration, and stays a UTC point when a UTC point is asked for.
package test {
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;
	private import Time::*;
	private import QuantityCalculations::*;

	part def Mission {
		attribute launch : TimeInstantValue = 5.0 [UTC];
		attribute afterBurn : TimeInstantValue = launch + 3.0 [s];
		attribute beforeBurn : TimeInstantValue = launch - 3.0 [s];
		attribute burnFirst : TimeInstantValue = 3.0 [s] + launch;
		attribute afterMinute : TimeInstantValue = launch + 1.0 [min];
		attribute afterHour : TimeInstantValue = launch + 1 [h];
		attribute burnTime : DurationValue = afterBurn - launch;
		attribute countdown : DurationValue = launch - afterBurn;
		attribute stillLaunch : TimeInstantValue = ConvertQuantity(launch, UTC);
		attribute launched : Boolean = afterBurn > launch;
		attribute onTime : Boolean = afterBurn == 8.0 [UTC];
		attribute notLate : Boolean = launch <= 5.0 [UTC];
		attribute earliest : TimeInstantValue = min(afterBurn, launch);
		attribute latest : TimeInstantValue = max(afterBurn, launch);
		attribute burns : DurationValue[2] = (burnTime, 2.0 [s]);
		attribute totalBurn : DurationValue = burns->sum();

		attribute doubled = 2 * launch;
		attribute perSecond = launch * 2.0 [s];
		attribute halved = launch / 2;
		attribute rate = 1.0 / launch;
		attribute negated = -launch;
		attribute summed = launch + afterBurn;
		attribute fromDuration = 10.0 [s] - launch;
		attribute againstDuration = launch == 5.0 [s];
		attribute instants : TimeInstantValue[2] = (launch, afterBurn);
		attribute instantSum = instants->sum();
	}
}
