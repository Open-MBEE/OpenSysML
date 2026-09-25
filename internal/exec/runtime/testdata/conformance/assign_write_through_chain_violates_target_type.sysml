package test {
	private import ScalarValues::*;

	// A write through a feature chain reaches a feature of the object the chain
	// names, and answers to that feature's declared type like any other write.
	part def Cell {
		attribute mark : Integer = 0;
	}

	part def Board {
		part cell : Cell;
		attribute raw;

		exhibit state run {
			entry; then go;

			state go {
				entry action set {
					assign raw := "x";
					assign cell.mark := raw;
				}
			}
		}
	}
}
