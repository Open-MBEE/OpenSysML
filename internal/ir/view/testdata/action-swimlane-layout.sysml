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


	// One performer with a lane in each of two exposed flows no Layout places.
	action runA {
		action load;
		then action ship;
	}
	action runB {
		action pack;
		then action post;
	}
	part station {
		part robot {
			perform runA.load;
			perform runB.pack;
		}
	}
	view sharedView : ActionFlowView {
		expose runA;
		expose runB;
		metadata Layout about station::robot { x = 320; y = 120; width = 220; height = 180; }
		metadata Layout about runA::load { x = 350; y = 190; }
		metadata Layout about runB::pack { x = 350; y = 190; }
	}

	// Both flows placed, and a node of the second.
	view pinnedView : ActionFlowView {
		expose runA;
		expose runB;
		metadata Layout about runA { x = 40; y = 40; width = 600; height = 400; }
		metadata Layout about runB { x = 700; y = 40; width = 600; height = 400; }
		metadata Layout about station::robot { x = 320; y = 120; width = 220; height = 180; }
		metadata Layout about runB::pack { x = 900; y = 200; }
	}

	// pinnedView with its first flow folded, and with its second.
	view pinnedFirstFoldedView : ActionFlowView {
		expose runA;
		expose runB;
		metadata Layout about runA { x = 40; y = 40; width = 600; height = 400; collapsed = true; }
		metadata Layout about runB { x = 700; y = 40; width = 600; height = 400; }
		metadata Layout about station::robot { x = 320; y = 120; width = 220; height = 180; }
		metadata Layout about runB::pack { x = 900; y = 200; }
	}
	view pinnedSecondFoldedView : ActionFlowView {
		expose runA;
		expose runB;
		metadata Layout about runA { x = 40; y = 40; width = 600; height = 400; }
		metadata Layout about runB { x = 700; y = 40; width = 600; height = 400; collapsed = true; }
		metadata Layout about station::robot { x = 320; y = 120; width = 220; height = 180; }
		metadata Layout about runB::pack { x = 900; y = 200; }
	}

	// The first of the two flows folded: its lane, the placed one, is not drawn.
	view foldedFlowView : ActionFlowView {
		expose runA;
		expose runB;
		metadata Layout about runA { collapsed = true; }
		metadata Layout about station::robot { x = 320; y = 120; width = 220; height = 180; }
		metadata Layout about runB::pack { x = 350; y = 190; }
	}

	// A Layout about the performer that only folds its lane.
	view foldedLaneView : ActionFlowView {
		expose runA;
		expose runB;
		metadata Layout about station::robot { collapsed = true; }
	}

	// A performer placed by a Layout of its own, with a lane in a flow and in a flow nested in it.
	action weld {
		action tack;
		action seam {
			action bead;
		}
		first tack then seam;
	}
	part torch {
		metadata Layout { x = 320; y = 120; }
		perform weld.tack;
		perform weld.seam.bead;
	}
}
