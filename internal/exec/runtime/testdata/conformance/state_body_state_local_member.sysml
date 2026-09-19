// A state's entry action resolves in that state's own scope, so a member the
// state declares answers a name the action uses, and the enclosing package's
// members stay visible through it.
package test {
	private import ScalarValues::*;

	attribute pkgBonus : Real = 1.0;

	state monitor {
		attribute result : Real = 0.0;

		entry; then start;
		state start;
		state working {
			attribute localGain : Real = 4.0;
			entry { assign result := localGain + pkgBonus; }
		}

		succession first start then working;
		succession first working then done;
	}
}
