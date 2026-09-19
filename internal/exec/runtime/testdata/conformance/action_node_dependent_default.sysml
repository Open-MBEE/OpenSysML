package test {
	private import ScalarValues::*;

	// A parameter default written in terms of an earlier parameter reads the
	// value the caller supplied for it: the supplied inputs are bound before the
	// defaults are evaluated, in declaration order. A supplied value still
	// replaces the default it would otherwise take.
	action def Pad {
		in a : Integer;
		in b : Integer = a * 2;
		in c : Integer = a + b;
		out result : Integer;

		first step;
		action step { assign result := a + b + c; }
	}

	action outer {
		out attribute defaulted : Integer;
		out attribute named : Integer;
		out attribute overridden : Integer;

		first start;
		then action byPosition = Pad(3);
		then action byName = Pad(a = 5);
		then action bothGiven = Pad(3, 10);
		then action fin {
			assign defaulted := byPosition;
			assign named := byName;
			assign overridden := bothGiven;
		}
		then done;
	}
}
