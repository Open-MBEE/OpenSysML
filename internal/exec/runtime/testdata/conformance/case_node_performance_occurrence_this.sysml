// A case run by no object holds the performances of its steps once `this` denotes it,
// as an action does: `this.one.v` reads what the performance of `one` wrote.
package test {
	private import ScalarValues::*;

	analysis def Late {
		subject s : Integer = 0;
		action one { out v : Integer[1]; assign v := 1; }
		action two { out v : Integer[1]; assign v := one.v + 1; }
		return r : Integer = this.one.v * 10 + this.two.v;
	}
}
