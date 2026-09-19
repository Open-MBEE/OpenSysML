// An attribute def specializing Array keeps its own members: the object answers
// Array's features from its shape and every other member from itself.
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

	calc def shape {
		return : String = grid.label;
	}
}
