// A positional `then` sequences from the nearest preceding feature, passing over
// the documentation, nested definition, import, multiplicity and comment written
// between: a runs, then b, then c, so the result is (1 + 1) * 10 + 3.
package test {
	private import ScalarValues::*;
	package Helpers {
		attribute def Marker;
	}
	action seq {
		attribute result : Integer = 1;

		first start;
		then action a { assign result := result + 1; }
		doc /* a runs before b */
		then action b { assign result := result * 10; }
		action def Inner;
		private import Helpers::*;
		multiplicity once [1];
		comment about a /* the first step */
		then action c { assign result := result + 3; }
		then done;
	}
}
