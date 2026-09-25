package Pictures {
	private import DocumentQueries::*;

	part def ImageReport :> Document {
		attribute redefines title = "Survey Plates";

		part survey : Section {
			attribute redefines title = "Survey";

			part plate : Image {
				attribute redefines location = "images/mark.png";
				attribute redefines caption = "Plate 1: the survey mark";
				attribute redefines alt = "a brass survey mark";
			}

			part remote : Image {
				attribute redefines location = "https://example.test/plates/plate2.png";
				attribute redefines caption = "Plate 2: from the archive";
			}
		}
	}
}
