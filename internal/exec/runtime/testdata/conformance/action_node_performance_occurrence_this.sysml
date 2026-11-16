// A definition run by no object is its own occurrence once `this` denotes it, and
// the performances of its nodes begun before that are its subperformances from then
// on: `this.one.v` reads what the performance of `one` holds.
package test {
	private import ScalarValues::*;

	action def Pipeline {
		out attribute seen : Integer[1] = 0;
		action one { out v : Integer[1]; assign v := 1; }
		action two { out v : Integer[1]; assign v := one.v + 1; }
		action fin { assign seen := this.one.v + this.two.v; }
		first start then one;
		first one then two;
		first two then fin;
		first fin then done;
	}
}
