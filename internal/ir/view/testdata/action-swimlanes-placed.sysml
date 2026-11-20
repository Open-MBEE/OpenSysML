package PlacedLanes {
	private import StandardViewDefinitions::*;
	private import DiagramLayout::*;

	action run {
		action load;
		action check;
		action ship;
		first start then load;
		first load then check;
		first check then ship;
	}
	part line {
		perform run;
		part robot {
			perform run.load;
			perform run.check;
		}
	}

	view placedView : ActionFlowView {
		expose run;
		metadata Layout about run { x = 40; y = 40; width = 600; height = 400; }
		metadata Layout about run::load { x = 300; y = 200; }
		metadata Layout about run::ship { x = 60; y = 150; }
	}
	view performedView : ActionFlowView {
		expose line::run;
		metadata Layout about line::run::load { x = 300; y = 200; }
	}
}
