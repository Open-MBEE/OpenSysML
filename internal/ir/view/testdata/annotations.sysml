package Annotated {
	private import ScalarValues::*;
	private import DiagramLayout::*;

	part def Pump {
		doc /* Moves fluid from the tank to the loop. */
		attribute rate : Real;
		comment /* Sized for the nominal flow. */
		comment Safety about Pump, Tank locale "en"
			/* Both are rated for the loop pressure. */
		rep inline language "plantuml" /* component Pump */
	}
	part def Tank {
		doc Capacity locale "en" /* Holds the working fluid. */
	}
	part def Valve {
		doc /* Closes the loop. */
		part seat : Seat {
			doc /* Seals the valve. */
		}
	}
	part def Seat;
}

package AnnotationViews {
	private import ScalarValues::Boolean;
	private import Views::*;
	private import StandardViewDefinitions::*;
	private import DiagramLayout::*;

	view tree {
		doc /* The structure of the loop, annotated. */
		expose Annotated::Pump;
		expose Annotated::Tank;
		render asTreeDiagram;
	}
	view definitions : GeneralView {
		expose Annotated::*[@SysML::Definition];
		filter @SysML::PartDefinition;
	}
	view unfiltered : GeneralView {
		expose Annotated::*;
	}
	view filtered {
		expose Annotated::Pump;
		filter @SysML::PartDefinition;
		render asTreeDiagram;
	}
	view exposedComment {
		expose Annotated::Pump::Safety;
		expose Annotated::Tank;
		render asTreeDiagram;
	}
	view exposedWithOwner {
		expose Annotated::Pump;
		expose Annotated::Pump::Safety;
		render asTreeDiagram;
	}
	view placed {
		expose Annotated::Valve;
		render asTreeDiagram;
		metadata Layout about Annotated::Valve { x = 20; y = 20; width = 200; height = 120; }
	}
	rendering noAnnotations :> Views::asTreeDiagram {
		attribute showAnnotations : Boolean default = false;
	}
	view empty {
		expose Annotated::Pump;
		filter @SysML::RequirementUsage;
	}
	view bare {
		expose Annotated::Pump;
		render noAnnotations;
		metadata Note about Annotated::Pump::Safety { text = "a note on the comment"; x = 300; y = 20; width = 120; height = 40; }
	}
}
