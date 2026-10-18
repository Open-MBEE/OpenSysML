// A verification definition and the usage typed by it are both owned by the
// part definition they test, and the usage binds its subject to `this`: the
// `this` of a performance an object owns is that object
// (`Parts::Part::ownedActions::this`), where `self` would be the case itself.
// Run on an object of the definition, the body reads the object's redefined cost
// against the limit a requirement usage of another package binds: pass.
package RequirementPkg {
	private import ScalarValues::Real;
	private import DefinitionPkg::Definition;

	requirement def CostRequirement {
		subject product : Definition;
		attribute maximumCost : Real;
		require constraint costOk { product.cost <= maximumCost }
	}
	requirement costReq : CostRequirement {
		:>> maximumCost = 100.0;
	}
}

package DefinitionPkg {
	private import ScalarValues::Real;
	private import RequirementPkg::costReq;

	part def Definition {
		attribute cost : Real;
		satisfy costReq;

		verification def CostTest {
			subject product : Definition;
			objective costObjective { verify costReq; }
			return verdict = VerificationCases::PassIf(product.cost <= RequirementPkg::costReq.maximumCost);
		}
		verification costTest : CostTest {
			subject product = this;
		}
	}
}

package ProductPkg {
	private import DefinitionPkg::Definition;

	part good : Definition {
		attribute :>> cost = 80.0;
	}
}
