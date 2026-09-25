package Numbered {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import Views::asElementTable;
	private import Views::asInterconnectionDiagram;

	part def Lens;
	part def Mirror;

	part def Telescope {
		part primary : Mirror;
		part corrector : Lens;
	}

	part scope : Telescope;

	view parts {
		expose scope::*;
		render asInterconnectionDiagram;
	}

	view roster {
		expose scope::*;
		render asElementTable;
	}

	calc def Parts :> Query {
		in root : Element;
		WhereType(source = Descendants(source = root, maxDepth = 2), type = "PartUsage")
	}

	part def NumberedReport :> Document {
		attribute redefines title = "Numbered Blocks";

		part optics : Section {
			attribute redefines title = "Optics";

			part masses : Table {
				attribute redefines caption = "Optical parts";
				calc rows : Parts { in root = scope; }
			}

			part layout : Diagram {
				attribute redefines caption = "How the parts connect";
				ref redefines source = parts;
			}

			part area : Formula {
				attribute redefines source = "A = \\pi r^2";
				attribute redefines caption = "Collecting area";
			}

			part photo : Image {
				attribute redefines location = "images/mark.png";
				attribute redefines alt = "the mark";
			}
		}

		part inventory : Section {
			attribute redefines title = "Inventory";

			part unnamed : Table {
				calc rows : Parts { in root = scope; }
			}

			part listed : Diagram {
				attribute redefines caption = "The parts as rows";
				ref redefines source = roster;
			}

			part plate : Image {
				attribute redefines location = "images/mark.png";
				attribute redefines caption = "The survey mark";
			}
		}
	}
}
