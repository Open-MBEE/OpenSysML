package test {
	// A calculation branching on its own parameter: the `if` selects which
	// `return` is reached, so only one branch is evaluated.
	calc sign {
		in n: Integer;
		if n < 0 {
			return : Integer = -1;
		}
		if n == 0 {
			return : Integer = 0;
		}
		return : Integer = 1;
	}
}
