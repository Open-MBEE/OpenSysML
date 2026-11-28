package LaneAlone {
	private import StandardViewDefinitions::*;
	private import DiagramLayout::*;

	action run {
		action load;
		action weld;
		action ship;
		first load then weld;
		first weld then ship;
	}
	part line {
		part robot {
			perform run.load;
		}
		part welder {
			perform run.weld;
		}
	}

	// A Layout about a performer and nothing else: what dragging a lane in a view with no Layout saves.
	view cornerView : ActionFlowView {
		expose run;
		metadata Layout about line::robot { x = 320; y = 120; }
	}

	view sizedView : ActionFlowView {
		expose run;
		metadata Layout about line::robot { x = 320; y = 120; width = 220; height = 180; }
	}
}
