// A merge passes on each value as it arrives: the first pass takes the value
// of its first input, and each later pass the value the loop returns.
package test {
	private import ScalarValues::*;

	action count {
		attribute n : Integer = 0;
		first start then zero;
		action zero { out y : Integer; assign y := 0; }
		merge again {
			in ref inputObject1 : Integer;
			in ref inputObject2 : Integer;
			out ref outputObject1 : Integer = (inputObject1, inputObject2);
		}
		action inc { in x : Integer; out y : Integer; assign y := x + 1; assign n := y; }
		first inc then check;
		decide check;
		if n < 3 then again;
		else done;
		succession flow of Integer from zero.y to again.inputObject1;
		succession flow of Integer from again.outputObject1 to inc.x;
		flow inc.y to again.inputObject2;
	}
}
