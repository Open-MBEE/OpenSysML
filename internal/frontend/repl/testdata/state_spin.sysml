// A machine that never settles: each state's completion transition leads back
// to the other, so a drain is only ever ended by a budget.
package Debug {
	state Spin {
		attribute n = 0;

		entry; then init;
		state init;
		state a {
			do {
				assign n := n + 1;
			}
		}
		state b {
			do {
				assign n := n + 1;
			}
		}

		succession first init then a;
		succession first a then b;
		succession first b then a;
	}
}
