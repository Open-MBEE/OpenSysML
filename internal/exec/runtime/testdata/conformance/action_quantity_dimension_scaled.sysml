// A write conforms whenever the unit measures in the dimension the target's
// quantity value type fixes, at any scale and however the unit was composed:
// minutes into a TimeValue, and a quotient of metres by seconds into a
// SpeedValue. The units are named qualified because a wildcard import is not in
// scope inside an action body.
package test {
	private import SI::*;
	private import ISQ::*;

	action convert {
		attribute t : ISQSpaceTime::TimeValue = 0.0 [s];
		attribute v : ISQSpaceTime::SpeedValue = 0.0 [m/s];
		attribute quick : ScalarValues::Boolean = false;

		first start;

		action write {
			assign t := 5.0 [SI::min];
			assign v := 10.0 [SI::m] / 2.0 [SI::s];
		}

		action check {
			assign quick := t > 60.0 [SI::s] and v > 1.0 [SI::m / SI::s];
		}

		done;

		succession first start then write;
		succession first write then check;
		succession first check then done;
	}
}
