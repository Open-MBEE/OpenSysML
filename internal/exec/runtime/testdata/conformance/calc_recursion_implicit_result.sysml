package test {
	// A body whose result is the value of its last expression, recursing: the
	// implicit result is the invocation's value the same way a `return` is.
	calc factorial {
		in n: Integer;
		if n <= 1 ? 1 else n * factorial(n - 1)
	}
}
