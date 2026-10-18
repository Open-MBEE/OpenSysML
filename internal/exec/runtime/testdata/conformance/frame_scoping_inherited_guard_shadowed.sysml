// A guard written in `Base` reads `Base::go`: a name resolves in the namespace
// the expression is written in (KerML 1.0 §8.2.3.5), and `Derived::go` is a
// second feature that duplicates the inherited name without redefining it, so a
// performance of `Derived` holds both and the inherited guard reads `Base::go`.
package test {
	private import ScalarValues::*;

	action def Base {
		attribute go : Boolean = true;
		attribute c : Integer := 0;
		first start;
		then action a;
		first a if go then b;
		action b { assign c := 1; }
	}

	action def Derived :> Base {
		attribute go : Boolean = false;
	}
}
