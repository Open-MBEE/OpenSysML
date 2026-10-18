// A `satisfy r;` a part definition states is inherited by every usage of it:
// validating the object of such a usage checks the requirement about that
// object, reading the usage's redefined value, so a cost under the limit holds.
// The requirement, the definition and the usage live in three packages, as a
// model split over three files would place them.
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
		attribute :>> cost = 80.0;
	}
}
