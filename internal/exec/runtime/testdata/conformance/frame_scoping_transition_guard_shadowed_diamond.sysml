// One declaration reached along two generalization paths is one feature: the
// machine holds a single `go`, which `Base`'s entry action writes and its
// transition reads, with no hidden copy beside it.
package test {
	private import ScalarValues::*;

	state def Base {
		attribute go : Boolean := false;
		attribute c : Integer := 0;

		entry assign go := true;
		then idle;
		state idle;
		state taken {
			entry assign c := 1;
		}

		transition first idle if go then taken;
	}

	state def Left :> Base;
	state def Right :> Base;
	state def Derived :> Left, Right;
}
