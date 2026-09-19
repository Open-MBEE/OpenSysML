// A body statement is evaluated in the scope it was written in, so the units of
// the document's imports resolve inside an action: this propagates a free-fall
// descent in ISQ quantities, and the magnitudes are those of the same
// integration written in bare reals (0.2 s steps from 500 m at -15 m/s under
// 1.62 m/s^2, ending on the step that puts h below zero).
package test {
	private import ScalarValues::*;
	private import SI::*;
	private import ISQ::*;

	action propagate {
		attribute t : ISQSpaceTime::TimeValue = 0.0 [s];
		attribute h : ISQBase::LengthValue = 500.0 [m];
		attribute v : ISQSpaceTime::SpeedValue = -15.0 [m/s];
		attribute dt : ISQSpaceTime::TimeValue = 0.2 [s];
		attribute g : ISQSpaceTime::AccelerationValue = 1.62 [m/s^2];

		first start;

		action integrate {
			while h > 0.0 [m] {
				assign v := v - g * dt;
				assign h := h + v * dt;
				assign t := t + dt;
			}
		}

		done;

		succession first start then integrate;
		succession first integrate then done;
	}
}
