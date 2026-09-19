package test {
	// Named arguments bind by parameter name, so they may appear in any order;
	// binding still follows parameter order.
	constraint scaledWithinLimit {
		test::scale(factor = 4, x = 3) == 12
	}

	calc scale {
		in x: Integer;
		in factor: Integer = 2;
		x * factor
	}
}
