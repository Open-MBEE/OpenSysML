package test {
	// A node written with an unbraced body must not swallow the enclosing
	// body's remaining members and the successions below belong to `outer`, so
	// `outer` still runs start -> bump -> done and `total` is 1.62.
	action outer {
		attribute total : Real = 0.0;

		first start;

		action bump assign total := total + 1.62;

		done;

		succession first start then bump;
		succession first bump then done;
	}
}
