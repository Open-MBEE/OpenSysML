// A verification case a part definition owns is a feature of every object of
// that definition: run on one of them, its body reads that object's features,
// so the owner's redefined cost is what the verdict is about. The object is the
// case's owner, not its subject, which the case declares none of.
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
