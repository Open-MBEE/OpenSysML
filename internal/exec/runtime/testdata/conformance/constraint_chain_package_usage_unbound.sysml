// A chain through a package-level requirement usage whose member nothing binds
// reads no value: the condition cannot be evaluated, and no value is invented.
package test {
	private import ScalarValues::*;

	requirement def CostRequirement {
		attribute maximumCost : Real;
		require constraint { maximumCost > 0.0 }
	}
	requirement costReq : CostRequirement;

	attribute budget : Real = 80.0;

	constraint withinBudget { budget <= costReq.maximumCost }
}
