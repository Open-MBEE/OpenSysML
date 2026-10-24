// A performance holding a hidden inherited feature still materializes its
// occurrence when a body reads through `this`: the occurrence's slots are keyed
// by name and hold the visible `go`; `Base::go` stays the performance's own.
package test {
	private import ScalarValues::*;

	action def Base {
		attribute go : Boolean = true;
		attribute c : Integer := 0;
		attribute seen : Integer := 7;
		first start;
		then action probe { assign seen := this.c; }
		then action a;
		first a if go then b;
		action b { assign c := 1; }
	}

	action def Derived :> Base {
		attribute go : Boolean = false;
	}
}
