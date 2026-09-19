package test {
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): both branches write
	// `winner` in the same step; the library leaves which write stands open, so
	// the executor reports the conflict and the value that stood.
	action race {
		attribute winner : String = "none";
		attribute hareRan : Boolean = false;
		attribute tortoiseRan : Boolean = false;

		first start;
		fork split;
		action hare { assign winner := "hare"; assign hareRan := true; }
		action tortoise { assign winner := "tortoise"; assign tortoiseRan := true; }
		join finish;
		done;

		succession first start then split;
		succession first split then hare;
		succession first split then tortoise;
		succession first hare then finish;
		succession first tortoise then finish;
		succession first finish then done;
	}
}
