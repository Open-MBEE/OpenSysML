// A plain flow and a succession flow deliver to one input of a merge: the token reaching
// it by control takes the plain flow's value, the succession flow's token its own.
package test {
	private import ScalarValues::*;
	action control {
		attribute ones : Integer = 0;
		attribute twos : Integer = 0;
		first start then split;
		fork split;
		first split then warm;
		first warm then settle;
		first settle then a;
		action warm;
		action settle;
		first split then b;
		action a { out y : Integer; assign y := 1; }
		action b { out y : Integer; assign y := 2; }
		first b then relay;
		first relay then m;
		action relay;
		merge m {
			in ref inputObject1 : Integer;
			in ref inputObject2 : Integer;
			out ref outputObject1 : Integer = (inputObject1, inputObject2);
		}
		succession flow of Integer from a.y to m.inputObject1;
		flow b.y to m.inputObject1;
		action keep {
			in v : Integer;
			if v == 1 { assign ones := ones + 1; }
			if v == 2 { assign twos := twos + 1; }
		}
		succession flow of Integer from m.outputObject1 to keep.v;
	}
}
