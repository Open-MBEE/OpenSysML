// A binding owned by the action between `a.x` and the single-valued `k` makes the
// values of `x` over all performances of `a` the one value of `k`: every
// performance reads 5.
package test {
	private import ScalarValues::*;

	action def BindIn {
		attribute k : Integer = 5;
		attribute total : Integer = 0;
		first start then a;
		action a[3] {
			in x : Integer;
			assign total := total + x;
		}
		bind a.x = k;
		succession first [*] a then [1] done;
	}
}
