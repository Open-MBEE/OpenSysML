package LaneStacked {
	private import StandardViewDefinitions::*;
	private import DiagramLayout::*;

	action runA {
		action load;
		action weld;
		action grind;
		action paint;
		first load then weld;
		first weld then grind;
		first grind then paint;
	}
	action runB {
		action pack;
		action post;
		first pack then post;
	}
	part line {
		part robot {
			perform runA.load;
		}
		part packer {
			perform runB.pack;
		}
	}

	// Two flows, the lane of the second placed under the lane of the first.
	view stackedView : ActionFlowView {
		expose runA;
		expose runB;
		metadata Layout about line::robot { x = 320; y = 120; }
		metadata Layout about line::packer { x = 320; y = 420; }
	}

    // The same, the lanes too close for the second flow's header to clear the first lane.
    view closeView : ActionFlowView {
        expose runA;
        expose runB;
        metadata Layout about line::robot { x = 320; y = 120; }
        metadata Layout about line::packer { x = 320; y = 330; }
    }
}
