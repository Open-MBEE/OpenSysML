package Anonymous {
	private import DocumentQueries::*;

	part def AnonymousReport :> Document {
		attribute redefines title = "Anonymous Sections";

		part : Section {
			attribute redefines title = "First";

			part : Paragraph {
				attribute redefines text = "In the first section.";
			}

			part : Section {
				attribute redefines title = "First nested";
			}

			part : Section {
				attribute redefines title = "Second nested";
			}
		}

		part : Section {
			attribute redefines title = "Second";
		}
	}
}
