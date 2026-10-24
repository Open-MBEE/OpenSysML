// An assignment written in `Base` writes `Base::go`, the feature its own
// namespace resolves the name to (KerML 1.0 §8.2.3.5), so in a performance of
// `Derived` it leaves `Derived::go` — a same-named feature that does not redefine
// it — as declared, and the guard `Base` declares reads what it wrote.
package test {
	private import ScalarValues::*;

	action def Base {
		attribute go : Boolean := true;
		attribute c : Integer := 0;
		first start;
		then assign go := false;
		then action a;
		first a if go then b;
		action b { assign c := 1; }
	}

	action def Derived :> Base {
		attribute go : Boolean := true;
	}
}
