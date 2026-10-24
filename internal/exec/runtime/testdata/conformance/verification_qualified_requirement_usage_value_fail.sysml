// A verification body reads a requirement usage's bound criterion in another package
// by qualified name and feature chain (`RequirementPkg::costReq.maximumCost`): fail.
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
	}
}

package ProductPkg {
	private import DefinitionPkg::Definition;
	private import RequirementPkg::costReq;

	part product : Definition {
		attribute :>> cost = 120.0;
	}

	verification def CostTest {
		subject p : Definition;
		objective costObjective { verify costReq; }
		return verdict = VerificationCases::PassIf(p.cost <= RequirementPkg::costReq.maximumCost);
	}
	verification costTest : CostTest { subject p = product; }
}
