// A verification body reads a criterion bound on a package-level requirement
// usage through a feature chain (`costReq.maximumCost`): no object stands for
// the usage, so the chain reads the value its member binds, as the qualified
// name `costReq::maximumCost` reads it. The subject's cost is under it: pass.
package test {
	private import ScalarValues::*;

	part def Product {
		attribute cost : Real;
	}

	requirement def CostRequirement {
		subject product : Product;
		attribute maximumCost : Real;
		require constraint { product.cost <= maximumCost }
	}
	requirement costReq : CostRequirement {
		:>> maximumCost = 100.0;
	}

	part product : Product {
		attribute :>> cost = 80.0;
	}

	verification def CostTest {
		subject p : Product;
		objective costObjective { verify costReq; }
		return verdict = VerificationCases::PassIf(p.cost <= costReq.maximumCost);
	}
	verification costTest : CostTest { subject p = product; }
}
