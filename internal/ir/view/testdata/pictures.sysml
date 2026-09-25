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

	// Pictures under and over the parts, interleaved: each layer keeps its
	// declaration order, the later picture over the earlier it overlaps.
	view layeredView {
		expose Site::Table;
		render asInterconnectionDiagram;
		@Canvas { unit = "px"; width = 900; height = 600; }
		metadata Layout about Site::Table::bench { x = 100; y = 200; width = 120; height = 60; }
		metadata Layout about Site::Table::camera { x = 400; y = 200; width = 120; height = 60; }
		@Picture { location = "images/bench.png"; x = 0; y = 0; width = 600; height = 400; }
		@Picture { location = "images/logo.png"; x = 100; y = 220; width = 80; height = 40; above = true; }
		@Picture { location = "images/site.jpg"; x = 300; y = 300; width = 400; height = 200; }
		@Picture { location = "images/logo.png"; x = 140; y = 240; width = 80; height = 40; above = true; }
	}

	// A picture under parts nothing places: the parts are drawn below it.
	view unplacedView {
		expose Site::Table;
		render asInterconnectionDiagram;
		@Picture { location = "images/bench.png"; x = 0; y = 0; width = 400; height = 300; }
	}

	// A picture alone: the view exposes no element.
	view pictureOnlyView {
		render asInterconnectionDiagram;
		@Picture { location = "images/site.jpg"; x = 0; y = 0; width = 640; height = 480; }
	}

	// A picture on a table: a table draws no picture, and says so.
	view tableView {
		expose Site::Bench;
		expose Site::Camera;
		render asElementTable;
		@Picture { location = "images/site.jpg"; x = 0; y = 0; width = 640; height = 480; }
	}

	// A picture at a URL: no drawing tool reads one, so it is not drawn.
	view remoteView {
		expose Site::Bench;
		render asInterconnectionDiagram;
		@Picture { location = "https://example.org/bench.png"; x = 0; y = 0; width = 400; height = 300; }
	}
}
