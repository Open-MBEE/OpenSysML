package test {
	// `%` is evaluated at invocation time, so it holds for parameter values and
	// not only for constants the folder can reach.
	calc remainder {
		in n: Integer;
		in d: Integer;
		n % d
	}
}
