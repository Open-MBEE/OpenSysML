package NoteFlow {
	private import ScalarValues::*;

	action def Outer {
		first start;
		action a {
			first start;
			action b;
			succession first start then b;
		}
		succession first start then a;
	}
}

package NoteViews {
	private import Views::*;
	private import StandardViewDefinitions::*;
	private import DiagramLayout::*;

	view nestedNoteView : ActionFlowView {
		expose NoteFlow::Outer;
		render asInterconnectionDiagram;
		@Canvas { unit = "px"; width = 400; height = 300; }
		metadata Layout about NoteFlow::Outer { x = 10; y = 10; width = 300; height = 200; }
		metadata Layout about NoteFlow::Outer::a { x = 20; y = 20; width = 100; height = 80; }
		metadata Note about NoteFlow::Outer::a { text = "watch"; x = 30; y = 30; }
	}
}
