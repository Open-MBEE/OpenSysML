// A verification case a part definition owns is a feature of every object of
// that definition, so a part held with a multiplicity gives one run per object
// it repeats, not one per usage: `-requirement` about no object runs the case
// on each of the four objects the fleet holds, two passing and two failing.
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

	part def Fleet {
		part cheap : Definition [2] {
			attribute :>> cost = 80.0;
		}
		part dear : Definition [2] {
			attribute :>> cost = 120.0;
		}
	}
	part fleet : Fleet;
}
