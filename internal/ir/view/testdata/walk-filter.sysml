metadata def Hidden;
metadata def Shown;

package FilteredWalk {
	private import Views::*;

	port def Signal;

	part def Radio {
		port rx : Signal;
	}

	part rover {
		attribute mass;
		part chassis {
			attribute length;
			#Shown part axle;
		}
		#Hidden part telemetry {
			port tx : Signal;
		}
		#Shown part radio : Radio;
		part gps {
			#Hidden port fix : Signal;
		}
		connect radio.rx to telemetry.tx;
		connect gps.fix to radio.rx;
	}

	// The filters admit the root and prune the walk: no attribute, nothing
	// marked Hidden, is drawn beneath it.
	view structure {
		expose rover;
		filter not @Hidden;
		filter @SysML::PartUsage or @SysML::PortUsage;
		render asTreeDiagram;
	}

	// The connectors to telemetry.tx and gps.fix end at pruned elements: each is
	// reported, not drawn, and not rerouted to the pruned element's owner.
	view links {
		expose rover;
		filter not @Hidden;
		render asInterconnectionDiagram;
	}

	// A whitelist: the recursive expose selects the marked elements and nothing
	// else, each a root of its own; the ancestors between them are not made up.
	view picked {
		expose rover::**[@Shown];
		render asTreeDiagram;
	}

	// A whitelist stated as a filter rejects the unmarked root, so the view is
	// empty: a filter prunes, it never adds the ancestors a marked element needs.
	view wholeRejected {
		expose rover;
		filter @Shown;
		render asTreeDiagram;
	}

	// A type filter prunes the ports a part has from its definition as well as
	// the ones it declares; neither is pinned.
	view partsOnly {
		expose rover;
		filter @SysML::PartUsage;
		render asInterconnectionDiagram;
	}
}
