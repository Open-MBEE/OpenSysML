// A transition declared in `Base` resolves its guard in `Base` (KerML 1.0
// §8.2.3.5.3), so `go` is `Base::go` there even in a machine of `Derived`, whose
// own `go` duplicates the inherited name without redefining it.
package test {
	private import ScalarValues::*;

	state def Base {
		attribute go : Boolean = true;
		attribute c : Integer := 0;

		entry; then idle;
		state idle;
		state taken {
			entry assign c := 1;
		}
		state skipped;

		transition first idle if go then taken;
		transition first idle if not go then skipped;
	}

	state def Derived :> Base {
		attribute go : Boolean = false;
	}
}
