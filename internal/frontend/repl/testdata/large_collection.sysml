// A part holding, through its rows, more members than a bounded lookup walks,
// each with an action to debug, so whether an identity beyond that bound is still
// held is observable.
package Farm {
	part def Cell {
		attribute count = 0;

		action tick {
			first t;
			action t {
				assign count := count + 1;
			}
		}
	}

	part def Row {
		part cells : Cell[1000];
	}

	part def Field {
		part rows : Row[3];
	}

	part field : Field;
	part spare : Cell;
}
