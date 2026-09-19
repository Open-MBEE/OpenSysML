package test {
	// A `return` inside a loop body unwinds the loop and the calculation at
	// once, so the elements after the first match are never visited.
	calc firstOver {
		in limit: Integer;
		attribute xs : Integer[*] = (1, 5, 9);
		for x in xs {
			if x > limit {
				return : Integer = x;
			}
		}
		return : Integer = 0;
	}
}
