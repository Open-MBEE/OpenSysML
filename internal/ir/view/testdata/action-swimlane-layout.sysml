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
}
