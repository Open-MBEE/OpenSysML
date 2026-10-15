// A require constraint reads a criterion bound on another package-level
// requirement usage through a feature chain (`costReq.maximumCost`), as the
// qualified name reads it; the budget is over the bound: violated.
package test {
	private import ScalarValues::*;

	requirement def CostRequirement {
		attribute maximumCost : Real;
		require constraint { maximumCost > 0.0 }
	}
	requirement costReq : CostRequirement {
		:>> maximumCost = 100.0;
	}

	attribute budget : Real = 120.0;

	requirement withinBudget {
		require constraint { budget <= costReq.maximumCost }
	}
}
