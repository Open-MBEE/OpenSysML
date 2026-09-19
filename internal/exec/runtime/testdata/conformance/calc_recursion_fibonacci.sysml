package test {
	// A recursion that reaches itself twice per invocation, so each invocation
	// binds its own parameters rather than sharing the caller's.
	calc fib {
		in n: Integer;
		return : Integer = if n < 2 ? n else fib(n - 1) + fib(n - 2);
	}
}
