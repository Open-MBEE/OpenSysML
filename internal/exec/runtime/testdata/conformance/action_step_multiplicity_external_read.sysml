// `a.x` read after every performance of `a` is the values of all three
// performances' `x` (ControlFunctions '.': nonunique), so its size is three and
// `c` is three in every order; `total` depends on how the performances'
// unordered statements interleave.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;
	private import NumericalFunctions::*;

	action def Reads {
		attribute c : Integer = 0;
		attribute total : Integer = 0;
		attribute n : Integer = 0;
		first start then a;
		action a[3] {
			out x : Integer;
			assign c := c + 1;
			assign x := c;
		}
		succession first [*] a then [1] q;
		action q {
			assign total := sum(a.x);
			assign n := size(a.x);
		}
		succession first q then done;
	}
}
