package test {
	// `if c ? a else b` evaluates only the branch its condition selects, so the
	// base case of a recursive calculation stops the recursion.
	calc countdown {
		in n: Integer;
		if n <= 0 ? 0 else countdown(n - 1)
	}
}
