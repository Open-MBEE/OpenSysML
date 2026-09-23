package Pump {
	private import DocumentQueries::*;

	state def Modes {
		view 'Pump Modes' : StandardViewDefinitions::StateTransitionView {
			expose Modes;
			@DiagramLayout::Canvas { unit = "px"; width = 300; height = 120; }
			metadata DiagramLayout::Layout about Idle { x = 20; y = 40; width = 100; height = 40; }
			metadata DiagramLayout::Layout about Running { x = 180; y = 40; width = 100; height = 40; }
			metadata DiagramLayout::Route about 'Idle accept Start then Running' { points = (120, 60, 180, 60); }
			render Views::asInterconnectionDiagram;
		}
		entry; then Idle;
		state Idle;
		state Running;
		transition 'Idle accept Start then Running' first Idle accept Start then Running;
	}

	action def Prime {
		view Priming : StandardViewDefinitions::ActionFlowView {
			expose Prime;
			@DiagramLayout::Canvas { unit = "px"; width = 140; height = 220; }
			metadata DiagramLayout::Layout about Fill { x = 20; y = 60; width = 100; height = 40; }
			metadata DiagramLayout::Layout about Vent { x = 20; y = 140; width = 100; height = 40; }
			metadata DiagramLayout::Route about 'Fill to Vent' { points = (70, 100, 70, 140); }
			render Views::asInterconnectionDiagram;
		}
		first start then Fill;
		action Fill;
		succession 'Fill to Vent' first Fill then Vent;
		action Vent;
		first Vent then done;
		action done terminate;
	}

	part def Handbook :> Document {
		attribute redefines title = "Pump Handbook";

		part behaviors : Section {
			attribute redefines title = "Behaviors";

			part modes : Diagram {
				attribute redefines caption = "Pump Modes";
				ref redefines source = Modes::'Pump Modes';
			}

			part priming : Diagram {
				attribute redefines caption = "Priming";
				ref redefines source = Prime::Priming;
			}
		}
	}
}
