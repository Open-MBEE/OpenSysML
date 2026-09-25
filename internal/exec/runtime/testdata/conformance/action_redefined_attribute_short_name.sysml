package test {
	private import ScalarValues::*;

	// A declared short name is a name: `attribute <sx> redefines x = 5;` answers
	// to `sx` and takes no name from what it redefines (KerML 7.3.4.5), so its
	// value binds under `sx` for the body to read.
	action run : Base {
		attribute <sx> redefines x = 5;
		attribute total : Integer = 0;

		first start;

		action bump {
			assign total := total + sx;
		}

		done;

		succession first start then bump;
		succession first bump then done;
	}

	action def Base {
		attribute x : Integer = 1;
	}
}
