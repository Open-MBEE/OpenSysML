package test {
	private import ScalarValues::*;

	// A node's frame nests as its flow does: the pin of a node inside a node's own
	// flow is read through both names, while the inner node still writes the
	// outermost action's feature by its bare name.
	action outer {
		attribute seen : Integer = 0;
		out attribute total : Integer;

		first start;
		then action leg {
			first start;
			then action inner {
				out v : Integer;
				assign v := 7;
				assign seen := seen + 1;
			}
			then action innerFin { assign seen := seen + inner.v; }
			then done;
		}
		then action fin { assign total := leg.inner.v + seen; }
		then done;
	}
}
