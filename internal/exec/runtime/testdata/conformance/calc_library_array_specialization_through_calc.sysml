// An Array specialization returned from a calc is still that specialization:
// the value keeps the object it was read from, so its own members stay readable.
package test {
	private import ScalarValues::*;
	private import Collections::*;

	attribute def LabeledGrid :> Array {
		attribute label : String;
		attribute scale : Real = 0.5;
	}

	attribute grid : LabeledGrid {
		:>> dimensions = (2, 2);
		:>> elements = (1, 2, 3, 4);
		:>> label = "grid";
	}

	calc def pick {
		return : LabeledGrid = grid;
	}

	calc def labelled {
		return : String = pick().label;
	}
}
