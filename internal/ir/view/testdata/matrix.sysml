package MatrixModel {
	private import RequirementDerivation::*;
	private import ModelingMetadata::Refinement;

	metadata def ProjectRefinement :> Refinement;
	metadata def ProjectDerivation :> DerivationMetadata;

	part logical {
		part ctrl;
	}
	part vehicle {
		part engine;
		port sourcePort;
		port targetPort;
		connection portLink connect sourcePort to targetPort;
		interface portInterface connect sourcePort to targetPort;
		satisfy r2;
	}
	requirement r1;
	requirement r2;
	requirement def DetailedRequirement;
	satisfy r1 by vehicle;
	satisfy requirement namedSatisfy : DetailedRequirement by vehicle;
	allocation chainAllocation allocate logical.ctrl to vehicle.engine;
	allocation namedAllocation allocate logical.ctrl to vehicle;
	dependency plainDependency from vehicle to r1;
	dependency refinementDependency from vehicle to r1 {
		@Refinement;
	}
	dependency specializedRefinement from vehicle to r1 {
		@ProjectRefinement;
	}
	#derivation connection {
		end :> derivedRequirements ::> r2;
		end :> originalRequirements ::> r1;
	}
	#derivation connection specializedDerivation {
		@ProjectDerivation;
		end :> derivedRequirements ::> r2;
		end :> originalRequirements ::> r1;
	}
	verification def VehicleVerification {
		objective {
			verify r1;
		}
	}
	verification vehicleVerification : VehicleVerification {
		objective {
			verify r1;
		}
	}
}

package EmptyModel {
    part unrelated;
    part secondUnrelated;
}

package MatrixViews {
	private import StandardViewDefinitions::*;

	view def AllRelations :> GridView {
		filter @SysML::SatisfyRequirementUsage or
			@SysML::VerificationCaseUsage or
			@SysML::AllocationUsage or
			@SysML::ConnectionUsage or
			@SysML::Dependency or
			@RequirementDerivation::DerivationMetadata or
			@ModelingMetadata::Refinement;
	}
	view allRelations : AllRelations { expose MatrixModel::**; }

	view allocationMatrix : GridView {
		filter @SysML::AllocationUsage;
		expose MatrixModel::**;
	}
	view shortNameGridView : grv {
		filter @SysML::AllocationUsage;
		expose MatrixModel::**;
	}
	view def InheritedAllocationMatrix :> GridView {
		filter @SysML::AllocationUsage;
	}
	view inheritedAllocationMatrix : InheritedAllocationMatrix { expose MatrixModel::**; }
	view directExposeFilterMatrix : GridView {
		expose MatrixModel::**[@SysML::Dependency];
	}
	view def InterfaceMatrix :> GridView {
		filter @SysML::InterfaceUsage;
	}
	view interfaceMatrix : InterfaceMatrix { expose MatrixModel::**; }
	view satisfySelectorMatrix : GridView {
		filter @SysML::SatisfyRequirementUsage;
		expose MatrixModel::**;
	}
	view verificationUsageMatrix : GridView {
		filter @SysML::VerificationCaseUsage;
		expose MatrixModel::**;
	}
	view connectionSelectorMatrix : GridView {
		filter @SysML::ConnectionUsage;
		expose MatrixModel::**;
	}
	view dependencySelectorMatrix : GridView {
		filter @SysML::Dependency;
		expose MatrixModel::**;
	}
	view refinementMetadataMatrix : GridView {
		filter @ModelingMetadata::Refinement;
		expose MatrixModel::**;
	}
	view derivationMetadataMatrix : GridView {
		filter @RequirementDerivation::DerivationMetadata;
		expose MatrixModel::**;
	}
	view def VerificationDefinitionMatrix :> GridView {
		filter @SysML::VerificationCaseDefinition;
	}
	view verificationDefinitionMatrix : VerificationDefinitionMatrix { expose MatrixModel::**; }
	view def SpecializedRefinementMatrix :> GridView {
		filter @MatrixModel::ProjectRefinement;
	}
	view specializedRefinementMatrix : SpecializedRefinementMatrix { expose MatrixModel::**; }
	view def SpecializedDerivationMatrix :> GridView {
		filter @MatrixModel::ProjectDerivation;
	}
	view specializedDerivationMatrix : SpecializedDerivationMatrix { expose MatrixModel::**; }

	view emptyMatrix : GridView {
		filter @SysML::AllocationUsage or @SysML::PartUsage;
		expose EmptyModel::*;
	}
	view nothingExposedMatrix : GridView {
		filter @SysML::AllocationUsage;
		expose EmptyModel::*;
	}
	view unknownSelectorTable : GridView {
		filter @SysML::PartUsage;
		expose EmptyModel::**;
	}
	view mixedSelectorMatrix : GridView {
		filter @SysML::AllocationUsage or @SysML::PartUsage;
		expose EmptyModel::*;
	}
	view negatedSelectorTable : GridView {
		filter not (@SysML::Dependency);
		expose MatrixModel::**;
	}
}
