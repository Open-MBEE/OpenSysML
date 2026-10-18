// A `satisfy r;` a part definition states is checked on each usage's object with
// the usage's redefined value; three packages as three files would place them: violated.
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

	part product : Definition {
		attribute :>> cost = 120.0;
	}
}
