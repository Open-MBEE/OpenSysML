// A transition declared in the state definition's body resolves its guard and
// effect in that namespace (KerML 1.0 §8.2.3.5.3): `level` is `Modes::level`, not
// the same-named attribute of the part exhibiting the machine nor the one the
// source state `idle` declares, however active that state is when the guard is
// read. The part's own `level` stays 7 beside it.
package test {
	private import ScalarValues::*;

	part def Controller {
		attribute level : Integer = 7;
		attribute seen : Integer = 0;

		state def Modes {
			attribute level : Integer = 1;

			entry; then idle;
			state idle {
				attribute level : Integer = 99;
			}
			state high;
			state low;

			transition first idle if level > 5 then high;
			transition first idle if level <= 5
				do { assign seen := level; }
				then low;
		}

		exhibit state modes : Modes;
	}
}
