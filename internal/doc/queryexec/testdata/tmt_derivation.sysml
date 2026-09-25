package Observatory {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	private import RequirementDerivation::*;
	private import ModelingMetadata::Refinement;

	requirement def MassRequirement;
	requirement def MirrorMassRequirement;
	requirement def SegmentMassRequirement;
	requirement def InstrumentMassRequirement;

	requirement systemMass : MassRequirement;
	requirement mirrorMass : MirrorMassRequirement;
	requirement segmentMass : SegmentMassRequirement;
	requirement instrumentMass : InstrumentMassRequirement;
	requirement cameraMass;
	requirement spectrographMass;

	requirement pointingAccuracy;
	requirement trackingAccuracy;
	requirement guidingAccuracy;
	requirement pointingBudget;
	requirement jitterBudget;
	requirement driftBudget;
	requirement thermalStability;
	requirement heaterPower;
	requirement opticsAlignment;
	requirement mirrorFigure;

	part mirrorDesign;
	part mountDesign;
	part guiderDesign;
	part thermalDesign;

	// Typed usage: the clause's first end is the original, the second the derived.
	connection massDerivation : Derivation connect systemMass to mirrorMass;

	// Semantic metadata on the usage, chaining a derived requirement further.
	#derivation connection connect mirrorMass to segmentMass;

	// N-ary clause: one original, several derived, in end order.
	#derivation connection connect (instrumentMass, cameraMass, spectrographMass);

	// Body ends stating their roles by metadata, written derived-first.
	#derivation connection {
		end #derive ::> trackingAccuracy;
		end #original ::> pointingAccuracy;
	}

	// Body ends stating their roles by subsetting the library's role features.
	connection : Derivation {
		end :> derivedRequirements ::> guidingAccuracy;
		end :> originalRequirements ::> pointingAccuracy;
	}

	// An end stating no role takes the one left over: derived beside a stated
	// original, original beside a stated derived.
	#derivation connection {
		end #original ::> thermalStability;
		end ::> heaterPower;
	}
	#derivation connection {
		end ::> opticsAlignment;
		end #derive ::> mirrorFigure;
	}

	// A derivation definition whose ends are typed by requirement definitions,
	// and a usage of it whose ends redefine the definition's by position.
	#derivation connection def BudgetDerivation {
		end #derive jitter : MirrorMassRequirement;
		end #original budget : MassRequirement;
		end #derive drift : InstrumentMassRequirement;
	}
	#derivation connection : BudgetDerivation {
		end jitter ::> jitterBudget;
		end budget ::> pointingBudget;
		end drift ::> driftBudget;
	}

	// The migrator's form: a definition specializing Derivation, ends
	// tagged with the qualified metadata names.
	connection def 'Derive Segment Mass Requirement' :> RequirementDerivation::Derivation {
		end #RequirementDerivation::original originalRequirement : MirrorMassRequirement;
		end #RequirementDerivation::derive derivedRequirement : SegmentMassRequirement;
	}

	// Definitions specializing a derivation definition: one redefines the derived
	// end, one only the original, one nothing; an untyped end relates no requirement.
	requirement def AlignmentRequirement;
	requirement def SegmentAlignmentRequirement;
	requirement def ActuatorAlignmentRequirement :> SegmentAlignmentRequirement;
	requirement def MirrorAlignmentRequirement :> AlignmentRequirement;
	connection def AlignmentDerivation :> Derivation {
		end #original alignment : AlignmentRequirement;
		end #derive segment : SegmentAlignmentRequirement;
	}
	connection def ActuatorAlignmentDerivation :> AlignmentDerivation {
		end :>> alignment;
		end actuator : ActuatorAlignmentRequirement :>> segment;
	}
	connection def MirrorAlignmentDerivation :> AlignmentDerivation {
		end mirror : MirrorAlignmentRequirement :>> alignment;
	}
	connection def InheritedAlignmentDerivation :> AlignmentDerivation;
	connection def UntypedDerivation :> Derivation {
		end #original alignment : AlignmentRequirement;
		end untyped;
	}

	// A plain connection between requirements is no derivation.
	connection connect systemMass to instrumentMass;

	// Refinement dependencies: prefix form, body form, and several clients
	// and suppliers; a plain dependency is no refinement.
	#refinement dependency pointingBudget to pointingAccuracy;
	dependency refinesMass from mirrorDesign to mirrorMass {
		@Refinement;
	}
	#refinement dependency from mountDesign, guiderDesign to trackingAccuracy, guidingAccuracy;
	dependency from thermalDesign to systemMass;

	calc def Related :> Query {
		in source : Element[1..*] ordered;
		in kind : String;
		in direction : String;
		in maxDepth : Integer;
		RelatedElements(
			source = source,
			relationshipKind = kind,
			direction = direction,
			maxDepth = maxDepth
		)
	}
}
