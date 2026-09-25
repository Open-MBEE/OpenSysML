package test {
	private import ScalarValues::*;

	// A step may be written as a redefinition of the one it overrides
	// (`action redefines bump;`), and answers to the name of what it redefines
	// (KerML 7.3.4.5), so a succession still orders it by that name.
	action run : Base {
		attribute total : Integer = 0;

		first start;

		action redefines bump {
			assign total := total + 3;
		}

		done;

		succession first start then bump;
		succession first bump then done;
	}

	action def Base {
		attribute total : Integer = 0;

		action bump;
	}
}
