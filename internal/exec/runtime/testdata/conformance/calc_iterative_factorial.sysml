package test {
	// An iterative calculation: locals, a `while` loop that updates them, and a
	// result parameter bound to the accumulator once the loop ends.
	calc factorial {
		in n: Integer;
		attribute acc : Integer = 1;
		attribute i : Integer = 1;
		while i <= n {
			assign acc := acc * i;
			assign i := i + 1;
		}
		return : Integer = acc;
	}
}
