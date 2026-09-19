package test {
	// Mutual recursion: neither calculation reaches its own base case without the
	// other, so the cycle between them is evaluated rather than refused.
	calc isEven {
		in n: Integer;
		return : Boolean = if n == 0 ? true else isOdd(n - 1);
	}
	calc isOdd {
		in n: Integer;
		return : Boolean = if n == 0 ? false else isEven(n - 1);
	}
}
