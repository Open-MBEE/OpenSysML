package test {
	// A recursive calculation: the conditional selects the base case, so the
	// recursion terminates and evaluates to its result.
	calc factorial {
		in n: Integer;
		return : Integer = if n <= 1 ? 1 else n * factorial(n - 1);
	}
}
