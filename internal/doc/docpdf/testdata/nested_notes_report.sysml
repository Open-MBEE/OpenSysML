package Nested {
	private import DocumentQueries::*;
	private import DiagramLayout::*;
	private import Views::*;

	// An action with a nested action of its own flow, a note anchored to the
	// nested one: the note's anchor names the drawn node, never the discarded
	// root of the nested lowering.
	action def Align {
		action value4 {
			action inner;
			first start;
			then inner;
		}
		action call;

		first start;
		then value4;
		then call;
	}

	comment about Align::value4 /* these values don't matter */

	view alignView : StandardViewDefinitions::ActionFlowView {
		expose Align;
		@Canvas { unit = "px"; width = 900; height = 400; }
		metadata Layout about Align::value4 { x = 42; y = 246; width = 113; height = 40; }
		metadata Layout about Align::call { x = 178; y = 183; width = 403; height = 38; }
		metadata Note about Align::value4 { text = "these values don't matter"; x = 728; y = 280; width = 217; height = 66; }
		metadata Note about Align::call { text = "doAcquisition"; x = 249; y = 154; width = 66; height = 14; }
	}

	part def NestedNotesReport :> Document {
		attribute redefines title = "Nested actions";

		part flow : Diagram {
			attribute redefines caption = "Align's nested flow, noted";
			ref redefines source = alignView;
		}
	}
}
