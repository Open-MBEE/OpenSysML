// Each guard reads the feature its own namespace resolves `go` to: the one
// `Base` declares reads `Base::go` and takes `b`, the one `Derived` declares
// reads `Derived::go` and takes `e`, in the same performance of `Derived`.
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
		attribute d : Integer := 0;
		first a if not go then e;
		action e { assign d := 2; }
	}
}
