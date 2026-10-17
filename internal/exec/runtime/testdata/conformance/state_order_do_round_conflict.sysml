package test {
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): the do behaviors of the
	// two regions of `work` are due in one round and both write `log`. The
	// library orders neither before the other, so `log` ends as 20 (a then b)
	// or 10 (b then a): order-dependent, and a default run reports it only up to
	// that order even though neither state has a transition out.
	state Machine {
		attribute log : Integer = 0;

		entry; then work;
		state work parallel {
			state a {
				entry; then a1;
				state a1 {
					do assign log := log + 10;
				}
			}
			state b {
				entry; then b1;
				state b1 {
					do assign log := log * 2;
				}
			}
		}
	}
}
