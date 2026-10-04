private import StandardViewDefinitions::*;
private import DocumentQueries::*;

package RelationshipMatrixExample {
	private import RequirementDerivation::*;
	private import ModelingMetadata::Refinement;

	requirement def PerformanceRequirement;
	requirement def BudgetRequirement;
	requirement performanceRequirement : PerformanceRequirement;
	requirement budgetRequirement : BudgetRequirement;

	part logicalDesign {
		part controller;
	}
	part vehicle {
		part engine;
		port powerOut;
		port powerIn;
		connection powerLink connect powerOut to powerIn;
		satisfy performanceRequirement;
	}
	part powerPlant;

	satisfy budgetRequirement by vehicle;
	satisfy requirement vehicleBudget : BudgetRequirement by vehicle;
	allocation controllerAllocation allocate logicalDesign.controller to vehicle.engine;
	allocation plantAllocation allocate logicalDesign.controller to powerPlant;
	dependency traceDependency from vehicle to performanceRequirement;
	dependency refinementDependency from vehicle to budgetRequirement {
		@Refinement;
	}
	#derivation connection {
		end :> originalRequirements ::> performanceRequirement;
		end :> derivedRequirements ::> budgetRequirement;
	}

	verification def PerformanceCheck;
	verification vehicleCheck : PerformanceCheck {
		objective {
			verify performanceRequirement;
		}
	}
}

view def RelationshipMatrix :> GridView {
	filter @SysML::SatisfyRequirementUsage or
		@SysML::VerificationCaseUsage or
		@SysML::AllocationUsage or
		@SysML::ConnectionUsage or
		@SysML::Dependency or
		@RequirementDerivation::DerivationMetadata or
		@ModelingMetadata::Refinement;
}
view relationshipMatrix : RelationshipMatrix {
	expose RelationshipMatrixExample::**;
}

part def RelationshipMatrixReport :> Document {
	attribute redefines title = "Relationship matrix example";

	part relationships : Diagram {
		attribute redefines caption = "Selected relationships among the exposed members";
		ref redefines source = relationshipMatrix;
	}
}
