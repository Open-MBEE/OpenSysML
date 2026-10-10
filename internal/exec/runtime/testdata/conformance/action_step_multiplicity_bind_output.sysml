// The two performances of `b` both output 7, so the values of `b.y` are the one
// value the binding gives `r`.
package test {
	private import ScalarValues::*;

	action def BindOut {
		attribute r : Integer[0..1];
		first start then b;
		action b[2] { out y : Integer = 7; }
		bind b.y = r;
		succession first [*] b then [1] done;
	}
}
