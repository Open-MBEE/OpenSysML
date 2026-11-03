// A part held with an open multiplicity contributes the objects it holds: one
// left at its lower bound of zero holds none and gives the definition-owned
// case no run, while one two usages subset gives a run on each of its two
// members, told apart by the verdict each cost decides.
package test {
	private import ScalarValues::Real;

	requirement def CostRequirement {
		subject product : Definition;
		attribute maximumCost : Real;
		require constraint costOk { product.cost <= maximumCost }
	}
	requirement costReq : CostRequirement {
		:>> maximumCost = 100.0;
	}

	part def Definition {
		attribute cost : Real;
		satisfy costReq;

		verification costTest {
			objective costObjective { verify costReq; }
			return verdict = VerificationCases::PassIf(costReq(this));
		}
	}

	part def Depot {
		part spares : Definition [0..*] {
			attribute :>> cost = 150.0;
		}
		part stock : Definition [0..*];
		part cheapStock : Definition :> stock {
			attribute :>> cost = 80.0;
		}
		part dearStock : Definition :> stock {
			attribute :>> cost = 120.0;
		}
	}
	part depot : Depot;
}
