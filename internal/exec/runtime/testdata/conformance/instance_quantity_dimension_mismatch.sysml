// A performed action writing the object's own feature answers to the quantity
// kind that feature declares, as a write in a plain action does: the shared
// conformance check is what every write path goes through.
package test {
	private import SI::*;
	private import ISQ::*;

	part def Host {
		attribute source = 3.0 [m / s];
		attribute t : ISQSpaceTime::TimeValue = 0.0 [s];

		perform action b {
			first start;

			action step {
				assign t := source;
			}

			done;

			succession first start then step;
			succession first step then done;
		}
	}
}
