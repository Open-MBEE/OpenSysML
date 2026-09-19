package test {
	// `for` iterates the sequence its collection evaluates to, binding each
	// element to the loop's variable in the body's own frame.
	calc total {
		in bias: Integer;
		attribute xs : Integer[*] = (2, 3, 4);
		attribute sum : Integer = bias;
		for x in xs {
			assign sum := sum + x;
		}
		return : Integer = sum;
	}
}
