package test {
	// Recursion terminates because the base case returns before the recursive
	// invocation is evaluated.
	calc factorial {
		in n: Integer;
		if n <= 1 {
			return : Integer = 1;
		}
		return : Integer = n * factorial(n - 1);
	}
}
