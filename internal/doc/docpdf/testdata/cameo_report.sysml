package Instrument {
	private import DocumentQueries::*;
	private import DiagramLayout::*;
	private import Views::*;

	attribute def Start;
	attribute def Stop;
	action def InitializePEAS;
	action def MonitorPEAS;

	// A state machine laid out the way a Cameo diagram of it was: every
	// state, pseudostate and transition positioned, one state in its own
	// colours, one note anchored on a state.
	state def PEAS {
		entry; then Init;
		state Init {
			do action : InitializePEAS;
		}
		state Ready {
			do action : MonitorPEAS;
		}
		state Fault;
		transition first Init then Ready;
		transition ready_stop first Ready accept Stop then done;
		transition fault first Ready accept Start then Fault;
	}

	comment aboutInit about PEAS::Init /* Runs once at power-up. */

	view peasStates : StandardViewDefinitions::StateTransitionView {
		expose PEAS;
		render asInterconnectionDiagram;
		@Canvas { unit = "px"; width = 540; height = 320; }
		metadata Layout about PEAS::Init { x = 60; y = 100; width = 140; height = 60; }
		metadata Layout about PEAS::Ready { x = 300; y = 100; width = 140; height = 60; }
		metadata Layout about PEAS::Fault { x = 300; y = 220; width = 140; height = 50; }
		metadata Route about PEAS::fault { points = (370, 160, 370, 220); }
		metadata Route about PEAS::ready_stop { points = (440, 130, 490, 130); }
		metadata Style about PEAS::Fault { fill = "#F2DCDB"; line = "#9C0006"; text = "#9C0006"; font = "Arial"; fontSize = 11; bold = true; }
		metadata Note about PEAS::Init { text = "Runs once at power-up."; x = 60; y = 200; width = 150; height = 40; }
	}

	part def CameoReport :> Document {
		attribute redefines title = "Instrument states";

		part states : Diagram {
			attribute redefines caption = "PEAS states, as Cameo drew them";
			ref redefines source = peasStates;
		}
	}
}
