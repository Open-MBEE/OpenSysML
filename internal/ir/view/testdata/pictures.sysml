package Site {
	private import Views::*;
	private import StandardViewDefinitions::*;
	private import DiagramLayout::*;

	part def Bench;
	part def Camera;

	part def Table {
		part bench : Bench;
		part camera : Camera;
	}

	// A picture drawn under the parts, at the bounds Cameo drew it at, and a
	// small one drawn over them.
	view mixedView {
		expose Site::Table;
		render asInterconnectionDiagram;
		@Canvas { unit = "px"; width = 900; height = 600; }
		metadata Layout about Site::Table::bench { x = 100; y = 200; width = 120; height = 60; }
		metadata Layout about Site::Table::camera { x = 400; y = 200; width = 120; height = 60; }
		@Picture { location = "images/bench.png"; x = 0; y = 0; width = 823; height = 577; alt = "the bench, from above"; }
		@Picture { location = "images/logo.png"; x = 700; y = 20; width = 80; height = 40; above = true; }
	}

	// A picture alone: the view exposes no element.
	view pictureOnlyView {
		render asInterconnectionDiagram;
		@Picture { location = "images/site.jpg"; x = 0; y = 0; width = 640; height = 480; }
	}
}
