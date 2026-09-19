// A packaged action whose nodes are named, so the debugger can break on one.
package Debug {
	action tally {
		attribute total = 0;

		first start;

		action accumulate {
			assign total := total + 5;
		}

		done;

		succession first start then accumulate;
		succession first accumulate then done;
	}
}
