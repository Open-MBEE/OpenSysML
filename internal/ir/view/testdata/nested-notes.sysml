package Nested {

	// An action with a nested action of its own flow: value4's body is lowered
	// in turn and drawn as the subtree of the node performing it.
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
}

package NestedViews {
	private import Views::*;
	private import StandardViewDefinitions::*;
	private import DiagramLayout::*;

	view alignView : ActionFlowView {
		expose Nested::Align;
		@Canvas { unit = "px"; width = 900; height = 400; }
		metadata Layout about Nested::Align::value4 { x = 42; y = 246; width = 113; height = 40; }
		metadata Layout about Nested::Align::call { x = 178; y = 183; width = 403; height = 38; }
		metadata Note about Nested::Align::value4 { text = "these values don't matter"; x = 728; y = 280; width = 217; height = 66; }
		metadata Note about Nested::Align::call { text = "doAcquisition"; x = 249; y = 154; width = 66; height = 14; }
	}
}
