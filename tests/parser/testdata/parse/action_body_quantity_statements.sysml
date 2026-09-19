// Quantities inside an action body: an attribute default, a loop condition and
// the assignments of a unit-carrying integration parse as quantity expressions
// like any other, which is what lets the runtime evaluate them in the scope the
// body was written in.
package descent {
	private import SI::*;
	private import ISQ::*;

	action propagate {
		attribute h : ISQBase::LengthValue = 500.0 [m];
		attribute v : ISQSpaceTime::SpeedValue = -15.0 [m/s];
		attribute dt : ISQSpaceTime::TimeValue = 0.2 [s];
		attribute g : ISQSpaceTime::AccelerationValue = 1.62 [m/s ** 2];

		first start;

		action integrate {
			while h > 0.0 [m] {
				assign v := v - g * dt;
				assign h := h + v * dt;
			}
		}

		done;

		succession first start then integrate;
		succession first integrate then done;
	}
}
