// A verification case a part definition owns is a feature of every object of
// that definition, and of nothing else: run with no object of the definition
// instantiated there is nothing its body could read cost from, so the run
// decides nothing — an inconclusive verdict asking for an object, as a
// definition's `satisfy` with no object asks for one — not an error about the
// value cost lacks. Nothing is wrong with the model.
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

	part cheap : Definition {
		attribute :>> cost = 80.0;
	}
}
