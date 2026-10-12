package test {
	private import ScalarValues::*;

	// The arguments come from an untyped calc, so the checker cannot tell the two
	// same-named actions apart: the run settles the call by the values, binding
	// them to the parameters of the action they fit exactly.
	package Tags {
		action def tag {
			in x : Integer;
			in y : Real;
			out code : Integer;
			first step;
			action step { assign code := 1; }
		}

		action def tag {
			in x : Real;
			in y : Integer;
			out code : Integer;
			first step;
			action step { assign code := 2; }
		}
	}

	calc def same { in v; v }

	action outer {
		private import Tags::*;
		attribute p = same(1.5);
		attribute q = same(2);
		out attribute chosen : Integer;

		first start;
		then action call = tag(x = p, y = q);
		then action fin { assign chosen := call.code; }
		then done;
	}
}
