// An action assignment whose value calls a library function and exponentiates.
package test {
	private import ScalarValues::*;
	private import RealFunctions::*;

	action tune {
		attribute gain : Real = 0.0;

		first start;

		action compute {
			assign gain := sqrt(16.0) + 2.0 ** 3;
		}

		done;

		succession first start then compute;
		succession first compute then done;
	}
}
