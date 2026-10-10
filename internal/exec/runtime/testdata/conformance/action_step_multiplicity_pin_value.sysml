// The feature value `in x = c` in the body of `a` is featured by each performance
// of `a`: each performance's `x` is 4 and its `y` 5, so the values of `a.y` over
// both performances sum to 10.
package test {
	private import ScalarValues::*;
	private import NumericalFunctions::*;

	action def Inc {
		in x : Integer;
		out y : Integer;
		first start then bump;
		action bump { assign y := x + 1; }
		then done;
	}

	action def PinValue {
		attribute c : Integer = 4;
		attribute total : Integer = 0;
		first start then a;
		action a : Inc[2] { in x = c; }
		succession first [*] a then [1] q;
		action q { assign total := sum(a.y); }
		succession first q then done;
	}
}
