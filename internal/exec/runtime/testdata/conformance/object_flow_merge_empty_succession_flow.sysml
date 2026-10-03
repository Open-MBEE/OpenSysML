// A succession flow that carries nothing reaches a merge beside a plain flow from the
// same action into the same input: the token takes the plain flow's value.
package test {
	private import ScalarValues::*;
	action control {
		attribute twos : Integer = 0;
		first start then p;
		action p {
			out x : Integer [0..1];
			out y : Integer;
			assign y := 2;
		}
		merge m {
			in ref inputObject1 : Integer;
			out ref outputObject1 : Integer = inputObject1;
		}
		succession flow of Integer from p.x to m.inputObject1;
		flow p.y to m.inputObject1;
		action keep {
			in v : Integer;
			if v == 2 { assign twos := twos + 1; }
		}
		succession flow of Integer from m.outputObject1 to keep.v;
	}
}
