package test {
	private import ScalarValues::*;

	// The parametric-budget pattern on an object: one calc usage, whose input
	// reads a slot of the object carrying it, feeding several of the object's
	// derived slots from one evaluation.
	calc def Mass {
		in n : Integer;
		out total = n + 2;
		out margin = total / 2;
	}

	part def Rig {
		attribute base = 8;

		calc m : Mass {
			in n = base;
		}

		attribute total = m.total;
		attribute margin = m.margin;
	}
}
