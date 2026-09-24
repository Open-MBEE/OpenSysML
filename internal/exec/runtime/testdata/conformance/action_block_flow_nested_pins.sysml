package test {
	private import ScalarValues::*;

	// A node declared in a branch may itself declare a node: the inner one is a
	// subperformance of the outer, its pin is read as `leg.inner.v` from the branch,
	// and the outer node's body reads it as `inner.v`. Both are reported under the
	// path of the performances holding them.
	action outer {
		out attribute total : Integer;

		first start;
		then action run {
			if true {
				action leg {
					out v : Integer;
					action inner { out v : Integer; assign v := 4; }
					assign v := inner.v + 1;
				}
				assign total := leg.v + leg.inner.v;
			}
		}
		then done;
	}
}
