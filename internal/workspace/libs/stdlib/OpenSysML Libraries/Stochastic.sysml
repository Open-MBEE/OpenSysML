library package Stochastic {
	doc
	/*
	 * NON-NORMATIVE. This package is an OpenSysML extension, not part of the OMG SysML
	 * standard library: it declares the metadata by which a model states the probability
	 * of a branch, which the SysML v2 notation has no other way to say. The vendored OMG
	 * files are unmodified. A tool that does not know the extension reads the metadata as
	 * any metadata usage and executes the branches as an unweighted choice.
	 *
	 * Random draws that a branch's probability or a random duration is decided by come
	 * from the RandomFunctions library, and are made only in a run given a seed.
	 */

	private import ScalarValues::Real;

	metadata def Probability {
		doc
		/*
		 * The probability that a run takes the annotated succession out of a decision
		 * node, among the alternatives that hold at that moment. It annotates the
		 * succession from within its body:
		 *
		 *   decide d;
		 *   first d then fast { @Probability { p = 0.7; } }
		 *   first d then slow { @Probability { p = 0.3; } }
		 *
		 * Either every succession out of one decision node carries a Probability or
		 * none does; p lies in 0.0..1.0; and where every p out of one node is a constant
		 * they sum to 1.0. A seeded run draws a branch by these weights, an unseeded
		 * run takes the most probable, and a weighted branch whose guard does not hold
		 * is left out of the draw, the others' weights renormalized. A transition out
		 * of a state cannot be weighted: one carrying a Probability is refused.
		 */
		attribute p : Real;
	}
}
