// The definition document of the case: the part def with its satisfy and owned verification.
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
