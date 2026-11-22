package Bench {
	private import DocumentQueries::*;
	private import KerML::Root::Element;

	part def Camera { port usb; }
	part def Hub { port socket; }
	part def System {
		part camera : Camera;
		part hub : Hub;
		connection link connect camera.usb to hub.socket;
	}

	calc def Connections :> Query {
		in root : Element;
		Project(
			source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "ConnectionUsage"),
			properties = ("name"),
			columns = (
				Column(name = "Ends", cell = { in row : KerML::Kernel::Connector; row.connectorEnd })
			)
		)
	}

	part def ConnectionReport :> Document {
		attribute redefines title = "Connections";

		part connections : Table {
			attribute redefines caption = "What each connection attaches";
			calc rows : Connections {
				in root = System;
			}
		}
	}
}
