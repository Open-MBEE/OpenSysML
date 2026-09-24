package Placed {
	private import DocumentQueries::*;
	private import DiagramLayout::*;
	private import Views::*;

	port def DataPort;

	part def Camera {
		port output : DataPort;
	}

	part def Recorder {
		port input : DataPort;
	}

	part imagingChain {
		part camera : Camera;
		part recorder : Recorder;
		part spare : Recorder;
		connection link connect camera.output to recorder.input;
	}

	// Positions two of the three parts; spare is left unplaced.
	view placedView {
		expose imagingChain;
		render asInterconnectionDiagram;
		@Canvas { unit = "px"; width = 400; height = 200; }
		metadata Layout about imagingChain::camera { x = 20; y = 40; width = 120; height = 60; }
		metadata Layout about imagingChain::recorder { x = 220; y = 40; width = 120; height = 60; }
	}

	part def PlacedReport :> Document {
		attribute redefines title = "Placed report";

		part chain : Diagram {
			attribute redefines caption = "Imaging chain, as laid out";
			ref redefines source = placedView;
		}
	}
}
