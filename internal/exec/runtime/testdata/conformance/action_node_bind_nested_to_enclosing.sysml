package test {
	private import ScalarValues::*;

	// A binding between a nested pin and a pin of the node enclosing it holds within
	// that node's own performance: `leg.inner.w = leg.x` reads the x the running leg
	// holds, and `leg.inner.v = leg.v` writes its v as inner ends, so leg's later node
	// and the action's `fin` read it there rather than a performance yet to come.
	action outer {
		attribute seven : Integer = 7;
		out attribute legV : Integer;
		out attribute own : Integer;

		bind leg.x = seven;
		bind leg.inner.w = leg.x;
		bind leg.inner.v = leg.v;

		first start;
		then action leg {
			in x : Integer;
			out v : Integer;
			first start;
			then action inner {
				in w : Integer;
				out v : Integer;
				assign v := w * 2;
			}
			then action after { assign own := v + 1; }
			then done;
		}
		then action fin { assign legV := leg.v; }
		then done;
	}
}
