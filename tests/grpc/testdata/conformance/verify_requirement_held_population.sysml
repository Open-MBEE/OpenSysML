package test {
	private import ScalarValues::Real;
	requirement def CostRequirement {
		subject product : Definition;
		attribute maximumCost : Real;
		require constraint costOk { product.cost <= maximumCost }
	}
	requirement costReq : CostRequirement { :>> maximumCost = 100.0; }
	part def Definition {
		attribute cost : Real;
		satisfy costReq;
		verification costTest {
			objective costObjective { verify costReq; }
			return verdict = VerificationCases::PassIf(costReq(this));
		}
	}
	part def Fleet { part cheap : Definition [2] { attribute :>> cost = 80.0; } }
	part fleet : Fleet;
}
