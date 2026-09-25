package test {
	private import ScalarValues::*;

	// A binding end may reach through a node into its own flow: `leg.inner.w` seeds
	// the pin of inner, a node of leg's flow, and `leg.inner.v` carries its result out.
	// leg's own `v` is a different pin from leg.inner.v, and a binding between two
	// nested pins (`leg.inner.v = leg.rest.n`) holds within leg's flow.
	action outer {
		attribute x : Integer = 7;
		out attribute seen : Integer;
		out attribute legV : Integer;
		out attribute restSaw : Integer;

		bind leg.inner.w = x;
		bind leg.inner.v = seen;
		bind leg.inner.v = leg.rest.n;

		first start;
		then action leg {
			out v : Integer;
			first start;
			then action inner {
				in w : Integer;
				out v : Integer;
				assign v := w * 2;
			}
			then action rest { in n : Integer; assign restSaw := n + 1; }
			then action own { assign v := 1; }
			then done;
		}
		then action fin { assign legV := leg.v; }
		then done;
	}
}
