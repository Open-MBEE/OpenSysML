package test {
	private import ScalarValues::*;

	// The two same-named actions the checker leaves tied name their outputs differently:
	// once the values settle a call, the node holds the pins of the action performed,
	// and is read as a value by its `out result` — the first candidate's pins count for nothing.
	package A {
		action def tag {
			in x : Integer;
			in y : Real;
			out mark : Integer;
			first step;
			action step { assign mark := 1; }
		}
	}

	package B {
		action def tag {
			in x : Real;
			in y : Integer;
			out result : Integer;
			first step;
			action step { assign result := 2; }
		}
	}

	calc def same { in v; v }

	action outer {
		private import A::*;
		private import B::*;
		attribute p = same(1.5);
		attribute q = same(2);
		out attribute chosen : Integer;
		out attribute marked : Integer;

		first start;
		then action byResult = tag(x = p, y = q);
		then action byMark = tag(x = q, y = p);
		then action fin { assign chosen := byResult; assign marked := byMark.mark; }
		then done;
	}
}
