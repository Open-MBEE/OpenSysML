package test {
	// A recursion far deeper than the fixed nesting bound that used to refuse
	// one: what stops a recursion is the run's budget, and this one terminates
	// well inside it.
	calc sumTo {
		in n: Integer;
		return : Integer = if n <= 0 ? 0 else n + sumTo(n - 1);
	}
}
