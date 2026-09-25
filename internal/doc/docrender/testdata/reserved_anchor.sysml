package Reserved {
	private import DocumentQueries::*;

	part def ReservedAnchorReport :> Document {
		attribute redefines title = "Reserved Anchors";

		part : Section {
			attribute redefines title = "Anonymous";
		}

		part section : Section {
			attribute redefines title = "Referenced";

			part : Paragraph {
				part back : Ref {
					attribute redefines text = "this section";
					ref redefines target = section;
				}
			}
		}
	}
}
