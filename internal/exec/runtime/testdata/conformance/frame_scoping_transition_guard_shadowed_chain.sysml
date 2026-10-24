// Two generals each declare `go`, neither redefining the other, and the machine
// declares none: it holds both, each general's transition reading its own
// (KerML 1.0 §8.2.3.5.3), and `go` in the machine's own namespace is `Mid::go`.
package test {
	private import ScalarValues::*;

	state def Base {
		attribute go : Boolean = true;
		attribute c : Integer := 0;

		entry; then idle;
		state idle;
		state taken {
			entry assign c := c + 1;
		}

		transition first idle if go then taken;
	}

	state def Mid :> Base {
		attribute go : Boolean = false;
		attribute d : Integer := 0;

		state skipped {
			entry assign d := 1;
		}

		transition first taken if not go then skipped;
	}

	state def Derived :> Mid;
}
