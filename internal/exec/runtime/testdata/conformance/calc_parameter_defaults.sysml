package test {
	// The second parameter receives no argument, so its declared default is
	// evaluated as part of binding.
	calc scale {
		in x: Integer;
		in factor: Integer = 3;
		x * factor
	}
}
