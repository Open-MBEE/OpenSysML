package test {
	private import ScalarValues::*;

	// A body attribute may be written as a redefinition of the one it overrides
	// (`attribute redefines x = 5;`). Such a declaration states no name of its
	// own: it answers to the name of what it redefines (KerML 7.3.4.5), so its
	// value must still bind for the body's statements to read.
	action run : Base {
		attribute redefines x = 5;
		attribute total : Integer = 0;

		first start;

		action bump {
			assign total := total + x;
		}

		done;

		succession first start then bump;
		succession first bump then done;
	}

	action def Base {
		attribute x : Integer = 1;
	}
}
