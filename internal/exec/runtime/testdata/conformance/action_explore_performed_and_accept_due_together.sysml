package test {
	private import SI::*;
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): one fork branch performs
	// `Sleeper`, whose own accept is due at 2 s, then writes `x := 1`; the other
	// branch's accept is due at the same instant, then writes `x := 2`. Both
	// accepts end at one reading of the clock and the library orders neither
	// branch against the other, so either write may be the last.
	action def Sleeper {
		first start;
		then action nap accept after 2 [s];
		then action rested;
	}

	action wake {
		attribute x : Integer = 0;

		first start;
		fork split;
		action performed : Sleeper;
		then action writeOne assign x := 1;
		action direct accept after 2 [s];
		then action writeTwo assign x := 2;
		join sync;
		done;

		succession first start then split;
		succession first split then performed;
		succession first split then direct;
		succession first writeOne then sync;
		succession first writeTwo then sync;
		succession first sync then done;
	}
}
