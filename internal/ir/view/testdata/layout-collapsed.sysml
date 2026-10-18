package FoldedViews {
	private import Views::*;
	private import DiagramLayout::*;

	port def Signal;

	part vehicle {
		part wheel {
			part hub;
		}
		part engine {
			port fuelIn : Signal;
			part piston;
			part pump {
				port out : Signal;
			}
		}
		part tank {
			port out : Signal;
		}
		connect tank.out to engine.fuelIn;
	}

	// A collapsed Layout folds engine's members away in every form; the tag stays.
	view folded {
		expose vehicle;
		render asTreeDiagram;
		metadata Layout about vehicle::wheel { x = 0; y = 40; }
		metadata Layout about vehicle::engine { x = 120; y = 40; collapsed = true; }
		metadata Layout about vehicle::tank { x = 240; y = 40; }
	}

	// Collapsed in an interconnection: engine keeps its port, and the connector
	// still ends at it; what engine nests is not drawn.
	view links {
		expose vehicle;
		render asInterconnectionDiagram;
		metadata Layout about vehicle::wheel { x = 0; y = 40; }
		metadata Layout about vehicle::engine { x = 120; y = 40; collapsed = true; }
		metadata Layout about vehicle::tank { x = 240; y = 40; }
	}

	// An exposed element under a collapsed exposed element is drawn as a root of
	// its own: the fold draws nothing beneath vehicle, so engine is not its descendant.
	view both {
		expose vehicle;
		expose vehicle::engine;
		render asTreeDiagram;
		metadata Layout about vehicle { x = 0; y = 0; collapsed = true; }
	}

	// A Layout binding only collapsed folds engine without placing it: the form
	// lays every node out itself, and nothing is left undrawn for want of a place.
	view foldOnly {
		expose vehicle;
		render asTreeDiagram;
		metadata Layout about vehicle::engine { collapsed = true; }
	}

	view foldOnlyLinks {
		expose vehicle;
		render asInterconnectionDiagram;
		metadata Layout about vehicle::engine { collapsed = true; }
	}

	// showParts = false hides the nested parts in the text form as in DOT.
	view noParts {
		expose vehicle;
		render asTreeDiagram;
		metadata Layout about vehicle { x = 0; y = 0; showParts = false; }
	}
}
