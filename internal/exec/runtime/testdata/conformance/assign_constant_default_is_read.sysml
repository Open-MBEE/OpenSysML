// A constant feature takes its value from its default when the performance is
// initialized, and reading it is no write: the constant
// keeps that value while another feature is assigned from it.
package test {
	private import ScalarValues::*;

	action tune {
		constant attribute limit : Integer default 5;
		attribute next : Integer = 0;

		first start;

		action step {
			assign next := limit + 1;
		}

		done;

		succession first start then step;
		succession first step then done;
	}
}
