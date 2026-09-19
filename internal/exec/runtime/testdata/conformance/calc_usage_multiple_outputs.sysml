package test {
	private import ScalarValues::*;

	// An invocation of a function yields exactly one result (KerML 7.4.9), so a
	// calc computing several values is consumed as a usage whose `out` features
	// are read by name: the usage binds the inputs, its body runs once, and each
	// output is a value readable from it.
	calc def Two {
		in n : Integer;
		out a = n + 1;
		out b = n * 2;
	}

	calc c : Two {
		in n = 5;
	}

	// An output of a calc usage as the value of an attribute: the pattern a
	// parametric budget is closed with, one usage feeding many features.
	attribute z = c.b;

	part def Rig {
		attribute base = 7;

		// A usage nested in a part definition, whose input names a sibling
		// attribute of that part.
		calc d : Two {
			in n = base;
		}

		attribute q = d.a;

		part inner {
			calc e : Two {
				in n = 10;
			}

			attribute deep = e.b;
		}
	}
}
