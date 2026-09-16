library package RandomFunctions {
	doc
	/*
	 * NON-NORMATIVE. This package is an OpenSysML extension, not part of the OMG KerML
	 * standard library: it declares functions whose result is a random draw from a stated
	 * distribution, which a model uses for a random duration (`accept after uniform(1, 80) [s]`)
	 * or a random value (`attribute d : Real = uniform(0.0, 1.0);`). The vendored OMG files
	 * are unmodified. A model that needs these writes `import RandomFunctions::*;`.
	 *
	 * A draw is made from the run's modeled-randomness stream, which the run's seed
	 * fixes: the same seed gives the same draws, and a run given no seed refuses the call
	 * rather than draw an unrepeatable value. Every draw is recorded in the run's witness,
	 * so a replay of the run makes the same draws without drawing again.
	 *
	 * The bounds may be quantities of one dimension (`uniform(1 [s], 80 [s])`), in which
	 * case the result is a quantity in the first bound's unit.
	 */

	public import ScalarValues::*;

	function uniform {
		doc
		/*
		 * A draw from the continuous uniform distribution over the closed interval
		 * lo..hi, every value of which is equally likely. Requires lo <= hi.
		 */
		in lo: Real[1]; in hi: Real[1]; return : Real[1];
	}

	function uniformInteger {
		doc
		/*
		 * A draw from the discrete uniform distribution over the integers lo..hi, both
		 * inclusive, each equally likely. Requires lo <= hi.
		 */
		in lo: Integer[1]; in hi: Integer[1]; return : Integer[1];
	}

	function triangular {
		doc
		/*
		 * A draw from the triangular distribution over lo..hi whose density peaks at
		 * mode. Requires lo <= mode <= hi and lo < hi.
		 */
		in lo: Real[1]; in mode: Real[1]; in hi: Real[1]; return : Real[1];
	}

	function normal {
		doc
		/*
		 * A draw from the normal distribution with the given mean and standard
		 * deviation sd. Requires sd >= 0.0; sd = 0.0 yields the mean.
		 */
		in mean: Real[1]; in sd: Real[1]; return : Real[1];
	}
}
