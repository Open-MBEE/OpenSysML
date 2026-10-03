// An object flow into a fork node lands on the fork's input object feature,
// and each output object feature passes the value on to its own successor.
package test {
	private import ScalarValues::*;

	action copy {
		first start then produce;
		action produce { out y : Real; assign y := 3.0; }
		fork split {
			in ref inputObject1 : Real;
			out ref outputObject1 : Real = inputObject1;
			out ref outputObject2 : Real = inputObject1;
		}
		action left { in v : Real; }
		action right { in v : Real; }
		succession flow of Real from produce.y to split.inputObject1;
		succession flow of Real from split.outputObject1 to left.v;
		succession flow of Real from split.outputObject2 to right.v;
	}
}
