package PlacedLane {
	private import StandardViewDefinitions::*;
	private import DiagramLayout::*;

	action run {
		action load;
		action ship;
		first load then ship;
	}
	part line {
		part robot {
			perform run.load;
		}
	}

	view laneView : ActionFlowView {
		expose run;
		metadata Layout about run { x = 40; y = 40; width = 600; height = 400; }
		metadata Layout about line::robot { x = 320; y = 120; width = 220; height = 180; }
	}

	// A lane and a node it holds placed in a flow no Layout places.
	view heldView : ActionFlowView {
		expose run;
		metadata Layout about line::robot { x = 320; y = 120; width = 220; height = 180; }
		metadata Layout about run::load { x = 350; y = 190; }
	}

	// One performer with a lane in a flow and in a flow nested in it.
	action assemble {
		action fetch;
		action build {
			action cut;
			then action weld;
		}
		first fetch then build;
	}
	part arm {
		perform assemble.fetch;
		perform assemble.build.cut;
	}
	view nestedView : ActionFlowView {
		expose assemble;
		metadata Layout about arm { x = 320; y = 120; }
	}
}
